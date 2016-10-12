// Copyright (c) 2016, Serge Bazanski <s@bazanski.pl>
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice,
// this list of conditions and the following disclaimer.
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation
// and/or other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
// AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
// IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
// ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
// LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
// CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
// SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
// CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
// ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

#include <time.h>
#include <ESP8266WiFi.h>
#include <WiFiClientSecure.h>

#include "secrets.h"

#define IO_SWITCH 5
#define IO_STATUS 4

const char *gatewayAddress = "iot.api.tog.ie";
const char *gatewayPath = "/api.tog.ie";
const char *gatewayFingerprint = "65 B8 24 2D 58 86 4F 38 AA 3D 05 FA A9 9D F7 F3 BA 7A A2 2C";


const char *targetAddress = "api.tog.ie";
const char *targetPath = "/api/1/switchifier/update";
char targetFingerprint[60];
const char *targetSecret = SECRETS_API_SECRET;
bool targetFingerprintValid = false;

const char* wifiSSID = SECRETS_WIFI_SSID;
const char* wifiPSK = SECRETS_WIFI_PSK;

bool updateSwitchState(bool state) {
  if (!targetFingerprintValid) {
    return false;
  }

  WiFiClientSecure client;
  if (!client.connect(targetAddress, 443)) {
    Serial.println("Could not connect to target.");
    delay(2000);
    return false;
  }
  if (!client.verify(targetFingerprint, targetAddress)) {
    Serial.println("Could not verify target fingerprint.");
    targetFingerprintValid = false;
    delay(2000);
    return false;
  }
  Serial.println("Connected to target and verified identity.");

  String PostData;
  if (state)
    PostData = String("value=true&secret=") + targetSecret;  
  else
    PostData = String("value=false&secret=") + targetSecret;  
  client.print(String("POST ") + targetPath + " HTTP/1.1\r\n" + 
    "Host: " + gatewayAddress + "\r\n" + 
    "User-Agent: SwitchifierArduinoESP8266\r\n" +
    "Content-Type: application/x-www-form-urlencoded\r\n" + 
    "Cache-Control: no-cache\r\n" +
    "Content-Length: ");
  client.println(PostData.length());
  client.println();
  client.println(PostData);

  String res = client.readStringUntil('\r');
  if (res == String("HTTP/1.1 200 OK")) {
    return true;
  } else {
    Serial.print("Received non-ok line: ");
    Serial.println(res);
    return false;
  }
}

bool getTargetFingerprint() {
  if (targetFingerprintValid) {
    return true;
  }
  WiFiClientSecure client;
  Serial.print("Getting Target fingerprint from ");
  Serial.println(gatewayAddress);
  if (!client.connect(gatewayAddress, 444)) {
    Serial.println("Could not connect.");
    delay(2000);
    return false;
  }
  if (!client.verify(gatewayFingerprint, gatewayAddress)) {
    Serial.println("Could not verify gateway fingerprint.");
    delay(2000);
    return false;
  }
  Serial.println("Connected to gateway and verified identity.");

  client.print(String("GET ") + gatewayPath + " HTTP/1.1\r\n" + 
    "Host: " + gatewayAddress + "\r\n" + 
    "User-Agent: SwitchifierArduinoESP8266\r\n" +
    "Connection: close\r\n\r\n");

  while (client.connected()) {
    String line = client.readStringUntil('\n');
    if (line == "\r") {
      Serial.println("Headers received.");
      break;
    }
  }
  String line = client.readStringUntil('\n');
  Serial.print("Received line: ");
  Serial.println(line);
  if (line.length() != 59) {
    Serial.print("Invalid length.");
    delay(2000);
    return false;
  }
  memcpy(targetFingerprint, line.c_str(), 59);
  targetFingerprintValid = true;
  Serial.println("Set target fingerprint.");

  return true;
}

void setup() {
  pinMode(IO_SWITCH, INPUT);
  pinMode(IO_STATUS, OUTPUT);
  configTime(0, 0, "pool.ntp.org", "time.nist.gov"); 
  Serial.begin(115200);
  Serial.setDebugOutput(true);
  Serial.println();
  Serial.print("connecting to ");
  Serial.println(wifiSSID);
  WiFi.begin(wifiSSID, wifiPSK);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.println("");
  Serial.println("WiFi connected");
  Serial.println("IP address: ");
  Serial.println(WiFi.localIP());
}

void loop() {
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("No wifi...");
    digitalWrite(IO_STATUS, 0);
    delay(500);
    return;
  }

  bool state = digitalRead(IO_SWITCH) == 0;
  Serial.print("Switch state: ");
  Serial.println(state);
  
  if (!getTargetFingerprint()) {
    digitalWrite(IO_STATUS, 0);
    return;
  }
  if (updateSwitchState(state))
  {
    digitalWrite(IO_STATUS, 1);
  } else {
    digitalWrite(IO_STATUS, 0);
  }
  delay(2000);
}
