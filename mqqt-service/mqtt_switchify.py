import requests
import json
import paho.mqtt.client as mqtt

LAST_STATE = 1
MQTT_SERVER = "0.0.0.0" # TODO pull from config


def update_switch(state):
    data ={
        'secret': "missing", # TODO pull from config
        'value': state,
    }

    r = requests.post('https://api.tog.ie/api/1/switchifier/update', data=data)
    print r


# The callback for when the client receives a CONNACK response from the server.
def on_connect(client, userdata, flags, rc):
    print("Connected with result code "+str(rc))

    # Subscribing in on_connect() means that if we lose the connection and
    # reconnect then subscriptions will be renewed.
    client.subscribe("/tog/sensors/knife_switch/+")


# The callback for when a PUBLISH message is received from the server.
def on_message(client, userdata, msg):
    global LAST_STATE
    #print(msg.topic+" "+str(msg.payload))
    switch = json.loads(msg.payload)
    state = switch.get("value")
    if state != LAST_STATE:
        print "Switch State Change: {0} != {1}".format(state, LAST_STATE)
        #We flip the switch value from the raw sensor data to a int representation of a bool
        #Switch Open = Sensor 1, Switchifier State 0
        #Switch Closed = Sensor 0, Switchifier State 1
        update_switch(int(not state))
        LAST_STATE = state


if __name__ == "__main__":
    client = mqtt.Client()
    client.on_connect = on_connect
    client.on_message = on_message
    client.connect(MQTT_SERVER, 1883, 60)
    client.loop_forever()

