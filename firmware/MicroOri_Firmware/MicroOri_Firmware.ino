/*
 * MicroOri 控制器固件 (Arduino Pro Micro / Leonardo / ATmega32U4)
 *
 * 接线:
 *   - D9  旋钮按钮 -> GND
 *   - D4  按钮    -> GND
 *   - D6  LED1（任一按下点亮）
 *   - D14 LED2（仅 D4 按钮按下时点亮）
 *
 * 功能:
 *   - 两个按钮分别映射一个可配置按键
 *   - 上位机通过 USB 串口(CDC) 读取/修改映射，配置存 EEPROM 持久化
 *   - 支持 F1-F12、浏览器收藏、浏览器搜索、A-Z
 *   - 支持左右 Ctrl/Shift/Alt/Win，以及 KC_PWR 电源管理键(ACPI)
 *
 * 串口协议(115200, 换行结尾):
 *   PING            -> PONG
 *   GET             -> MAP knob <name> / MAP button <name> / OK
 *   SET knob <name> -> OK / ERR <msg>
 *   SET button <name> -> OK / ERR <msg>
 *   RESET           -> 恢复默认(F8/F9)
 *
 * 需要 NicoHood 的 HID 库 (HID-Project)。
 */
#include "HID-Project.h"
#include <EEPROM.h>

// ---------------- 按键表 ----------------
enum KeyId : uint8_t {
  K_NONE = 0,
  K_F1 = 1, K_F2 = 2, K_F3 = 3, K_F4 = 4, K_F5 = 5, K_F6 = 6,
  K_F7 = 7, K_F8 = 8, K_F9 = 9, K_F10 = 10, K_F11 = 11, K_F12 = 12,
  K_FAV = 13,   // 浏览器收藏 Browser Favorites
  K_SEARCH = 14, // 浏览器搜索 Browser Search
  K_A = 15, K_B, K_C, K_D, K_E, K_F, K_G, K_H, K_I, K_J,
  K_K, K_L, K_M, K_N, K_O, K_P, K_Q, K_R, K_S, K_T,
  K_U, K_V, K_W, K_X, K_Y, K_Z,
  K_LCTRL, K_RCTRL, K_LSHIFT, K_RSHIFT, K_LALT, K_RALT,
  K_LWIN, K_RWIN, K_PWR
};

// ---------------- 引脚 ----------------
const uint8_t PIN_KNOB = 9;     // 旋钮按钮 D9
const uint8_t PIN_BUTTON = 4;   // 按钮 D4
const uint8_t LED_PIN = 6;      // D6
const uint8_t LED_PIN2 = 14;    // D14
const uint8_t RX_LED = 17;      // Pro Micro 板载 RX 灯

// ---------------- EEPROM ----------------
const uint8_t EE_MAGIC = 0x4D;
const uint16_t ADDR_MAGIC = 0;
const uint16_t ADDR_KNOB = 1;
const uint16_t ADDR_BUTTON = 2;

const KeyId DEFAULT_KNOB = K_F8;
const KeyId DEFAULT_BUTTON = K_F9;

// ---------------- 状态 ----------------
KeyId knobKey = K_NONE;
KeyId buttonKey = K_NONE;
KeyId heldKnob = K_NONE;
KeyId heldButton = K_NONE;
bool announced = false;
String serialLine = "";

// ---------------- 名字转换 ----------------
// 生成 F1..F12 的规范名（不带前导零），与上位机保持一致。
const char* fnName(uint8_t n) {
  static char buf[4];
  if (n < 10) {
    buf[0] = 'F';
    buf[1] = '0' + n;
    buf[2] = 0;
  } else {
    buf[0] = 'F';
    buf[1] = '0' + (n / 10);
    buf[2] = '0' + (n % 10);
    buf[3] = 0;
  }
  return buf;
}

const char* keyName(KeyId id) {
  if (id >= K_F1 && id <= K_F12) {
    return fnName(id - K_F1 + 1);
  }
  if (id == K_FAV) return "FAV";
  if (id == K_SEARCH) return "SEARCH";
  if (id >= K_A && id <= K_Z) {
    static char c[2] = { 'A', 0 };
    c[0] = 'A' + (id - K_A);
    return c;
  }
  if (id == K_LCTRL) return "LCTRL";
  if (id == K_RCTRL) return "RCTRL";
  if (id == K_LSHIFT) return "LSHIFT";
  if (id == K_RSHIFT) return "RSHIFT";
  if (id == K_LALT) return "LALT";
  if (id == K_RALT) return "RALT";
  if (id == K_LWIN) return "LWIN";
  if (id == K_RWIN) return "RWIN";
  if (id == K_PWR) return "KC_PWR";
  return "NONE";
}

KeyId keyFromName(const String& name) {
  if (name == "NONE") return K_NONE;
  if (name == "FAV") return K_FAV;
  if (name == "SEARCH") return K_SEARCH;
  for (uint8_t i = 0; i < 12; i++) {
    if (name == fnName(i + 1)) return (KeyId)(K_F1 + i);
  }
  if (name.length() == 1 && name[0] >= 'A' && name[0] <= 'Z') {
    return (KeyId)(K_A + (name[0] - 'A'));
  }
  if (name == "LCTRL" || name == "LEFT_CTRL") return K_LCTRL;
  if (name == "RCTRL" || name == "RIGHT_CTRL") return K_RCTRL;
  if (name == "LSHIFT" || name == "LEFT_SHIFT") return K_LSHIFT;
  if (name == "RSHIFT" || name == "RIGHT_SHIFT") return K_RSHIFT;
  if (name == "LALT" || name == "LEFT_ALT") return K_LALT;
  if (name == "RALT" || name == "RIGHT_ALT") return K_RALT;
  if (name == "LWIN" || name == "LEFT_WIN" || name == "LGUI") return K_LWIN;
  if (name == "RWIN" || name == "RIGHT_WIN" || name == "RGUI") return K_RWIN;
  if (name == "KC_PWR" || name == "POWER") return K_PWR;
  return K_NONE;
}

// ---------------- 按键输出 ----------------
bool keyPress(KeyId id) {
  if (id >= K_F1 && id <= K_F12) {
    Keyboard.press((KeyboardKeycode)(KEY_F1 + (id - K_F1)));
    return true;
  }
  if (id == K_FAV) { Consumer.press(CONSUMER_BROWSER_BOOKMARKS); return true; }
  if (id == K_SEARCH) { Consumer.press(HID_CONSUMER_AC_SEARCH); return true; }
  if (id >= K_A && id <= K_Z) {
    Keyboard.press((KeyboardKeycode)(0x04 + (id - K_A)));
    return true;
  }
  if (id == K_LCTRL) { Keyboard.press(KEY_LEFT_CTRL); return true; }
  if (id == K_RCTRL) { Keyboard.press(KEY_RIGHT_CTRL); return true; }
  if (id == K_LSHIFT) { Keyboard.press(KEY_LEFT_SHIFT); return true; }
  if (id == K_RSHIFT) { Keyboard.press(KEY_RIGHT_SHIFT); return true; }
  if (id == K_LALT) { Keyboard.press(KEY_LEFT_ALT); return true; }
  if (id == K_RALT) { Keyboard.press(KEY_RIGHT_ALT); return true; }
  if (id == K_LWIN) { Keyboard.press(KEY_LEFT_GUI); return true; }
  if (id == K_RWIN) { Keyboard.press(KEY_RIGHT_GUI); return true; }
  if (id == K_PWR) { System.press(SYSTEM_POWER_DOWN); return true; }
  return false;
}

bool keyRelease(KeyId id) {
  if (id >= K_F1 && id <= K_F12) {
    Keyboard.release((KeyboardKeycode)(KEY_F1 + (id - K_F1)));
    return true;
  }
  if (id == K_FAV) { Consumer.release(CONSUMER_BROWSER_BOOKMARKS); return true; }
  if (id == K_SEARCH) { Consumer.release(HID_CONSUMER_AC_SEARCH); return true; }
  if (id >= K_A && id <= K_Z) {
    Keyboard.release((KeyboardKeycode)(0x04 + (id - K_A)));
    return true;
  }
  if (id == K_LCTRL) { Keyboard.release(KEY_LEFT_CTRL); return true; }
  if (id == K_RCTRL) { Keyboard.release(KEY_RIGHT_CTRL); return true; }
  if (id == K_LSHIFT) { Keyboard.release(KEY_LEFT_SHIFT); return true; }
  if (id == K_RSHIFT) { Keyboard.release(KEY_RIGHT_SHIFT); return true; }
  if (id == K_LALT) { Keyboard.release(KEY_LEFT_ALT); return true; }
  if (id == K_RALT) { Keyboard.release(KEY_RIGHT_ALT); return true; }
  if (id == K_LWIN) { Keyboard.release(KEY_LEFT_GUI); return true; }
  if (id == K_RWIN) { Keyboard.release(KEY_RIGHT_GUI); return true; }
  if (id == K_PWR) { System.release(); return true; }
  return false;
}

// ---------------- 配置持久化 ----------------
void saveConfig() {
  EEPROM.write(ADDR_MAGIC, EE_MAGIC);
  EEPROM.write(ADDR_KNOB, (uint8_t)knobKey);
  EEPROM.write(ADDR_BUTTON, (uint8_t)buttonKey);
}

void loadConfig() {
  if (EEPROM.read(ADDR_MAGIC) == EE_MAGIC) {
    knobKey = (KeyId)EEPROM.read(ADDR_KNOB);
    buttonKey = (KeyId)EEPROM.read(ADDR_BUTTON);
  } else {
    knobKey = DEFAULT_KNOB;
    buttonKey = DEFAULT_BUTTON;
    saveConfig();
  }
  // 防止损坏数据
  if (knobKey > K_PWR) knobKey = K_NONE;
  if (buttonKey > K_PWR) buttonKey = K_NONE;
}

void sendMap() {
  Serial.print(F("MAP knob "));
  Serial.println(keyName(knobKey));
  Serial.print(F("MAP button "));
  Serial.println(keyName(buttonKey));
  Serial.println(F("OK"));
}

// ---------------- 按钮处理 ----------------
void updateButton(uint8_t pin, KeyId assigned, KeyId& held) {
  bool pressed = (digitalRead(pin) == LOW);
  if (pressed && held == K_NONE) {
    keyPress(assigned);
    held = assigned;
  } else if (!pressed && held != K_NONE) {
    keyRelease(held);
    held = K_NONE;
  }
}

// ---------------- 串口命令 ----------------
void processCommand(const String& line) {
  if (line == "PING") {
    Serial.println(F("PONG"));
    return;
  }
  if (line == "GET") {
    sendMap();
    return;
  }
  if (line == "RESET") {
    knobKey = DEFAULT_KNOB;
    buttonKey = DEFAULT_BUTTON;
    if (heldKnob != K_NONE) { keyRelease(heldKnob); heldKnob = K_NONE; }
    if (heldButton != K_NONE) { keyRelease(heldButton); heldButton = K_NONE; }
    saveConfig();
    Serial.println(F("OK"));
    return;
  }
  if (line.startsWith("SET ")) {
    String rest = line.substring(4);
    int sp = rest.indexOf(' ');
    if (sp < 0) {
      Serial.println(F("ERR bad command"));
      return;
    }
    String target = rest.substring(0, sp);
    String name = rest.substring(sp + 1);
    name.trim();
    KeyId id = keyFromName(name);
    if (target == "knob") {
      if (heldKnob != K_NONE) { keyRelease(heldKnob); heldKnob = K_NONE; }
      knobKey = id;
      saveConfig();
      Serial.println(F("OK"));
      return;
    }
    if (target == "button") {
      if (heldButton != K_NONE) { keyRelease(heldButton); heldButton = K_NONE; }
      buttonKey = id;
      saveConfig();
      Serial.println(F("OK"));
      return;
    }
    Serial.println(F("ERR bad target"));
    return;
  }
  Serial.println(F("ERR unknown"));
}

void handleSerial() {
  while (Serial.available()) {
    char c = (char)Serial.read();
    if (c == '\n') {
      serialLine.trim();
      if (serialLine.length() > 0) processCommand(serialLine);
      serialLine = "";
    } else if (c != '\r') {
      serialLine += c;
      if (serialLine.length() > 64) serialLine = "";  // 防止异常输入
    }
  }

  if (Serial && !announced) {
    announced = true;
    Serial.println(F("MICROORI 1"));
    sendMap();
  }
}

// ---------------- 主流程 ----------------
void setup() {
  pinMode(PIN_KNOB, INPUT_PULLUP);
  pinMode(PIN_BUTTON, INPUT_PULLUP);
  pinMode(LED_PIN, OUTPUT);
  pinMode(LED_PIN2, OUTPUT);
  digitalWrite(LED_PIN, LOW);
  digitalWrite(LED_PIN2, LOW);

  // 关闭板载 RX/TX 灯
  pinMode(RX_LED, OUTPUT);
  digitalWrite(RX_LED, HIGH);
  TXLED0;

  Keyboard.begin();
  Consumer.begin();
  System.begin();
  Serial.begin(115200);

  loadConfig();
}

void loop() {
  handleSerial();

  updateButton(PIN_KNOB, knobKey, heldKnob);
  updateButton(PIN_BUTTON, buttonKey, heldButton);

  bool knobPressed = (digitalRead(PIN_KNOB) == LOW);
  bool buttonPressed = (digitalRead(PIN_BUTTON) == LOW);
  digitalWrite(LED_PIN, knobPressed || buttonPressed ? HIGH : LOW);
  digitalWrite(LED_PIN2, buttonPressed ? HIGH : LOW);
}
