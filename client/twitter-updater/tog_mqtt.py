# -*- coding: utf-8 -*-
# Fer Uría <fauria@gmail.com>
# License: WTFPL
import logging
import paho.mqtt.client as mqtt
import json  # noqa
import twitter
import time  # noqa
from os import environ as env

"""
TOG Knife Switch.
Updates a Twitter account bio with a value read from an MQTT queue.

Usage: python tog_switch.py

Environment variables:
* LOG_LEVEL: Python logging module level. Default: INFO.
* TWITTER_CONSUMER_KEY: Twitter API consumer key. Default: None.
* TWITTER_CONSUMER_SECRET: Twitter API consumer secret. Default: None.
* TWITTER_ACCESS_TOKEN: Twitter API access token. Default: None.
* TWITTER_ACCESS_TOKEN_SECRET: Twitter API access token secret. Default: None.
* TWITTER_ID: Twitter user id. Default: 76108817.
* TWITTER_BIO_MESSAGE: What will be added to the bio. Default: ' Space is: '
* MQTT_HOST: IP address of MQTT server. Default: 10.48.1.254
* MQTT_PORT: Port for MQTT. Default: 1883
* MQTT_KEEPALIVE: Keepalive value. Default: 60.
* MQTT_TOPIC: Topic to subscribe. Default: /tog/sensors/knife_switch/knife_switch_ca750200  # noqa: E501
* TOG_SWITCH_STATUS: Initial switch status. Default: None.
* TOG_OPEN_STATUS: Status when 0. Default: OPEN.
* TOG_CLOSED_STATUS: Status when 1. Default: CLOSED.

"""
logging.basicConfig()
logger = logging.getLogger(__name__)
logger.setLevel(getattr(logging, env.get('LOG_LEVEL', 'INFO')))


class TOGKnifeSwitch(object):
    def __init__(self, kwargs):
        self.__dict__.update(kwargs)

        self.space_states = (
            self.tog_open_status,  # Status 0
            self.tog_closed_status  # Status 1
        )
        self.twitter = twitter.Api(consumer_key=self.twitter_consumer_key,
                                   consumer_secret=self.twitter_consumer_secret,  # noqa: E501
                                   access_token_key=self.twitter_access_token_key,  # noqa: E501
                                   access_token_secret=self.twitter_access_token_secret)  # noqa: E501
        logger.info('Twitter API connected.')
        self.mqtt = mqtt.Client()
        self.mqtt.connect(self.mqtt_host,
                          self.mqtt_port,
                          self.mqtt_keepalive)

    def __on_connect(self, client, userdata, flags, rc):
        logger.info('MQTT connected with result: {}'.format(str(rc)))
        self.mqtt.subscribe(self.mqtt_topic)
        logger.info('MQTT topic subscribed: {}'.format(self.mqtt_topic))

    def __on_message(self, client, userdata, msg):
        payload = json.loads(msg.payload)
        current_status = payload.get('value', None)
        logger.debug('Current switch status: {}'.format(str(current_status)))
        if self.tog_switch_status != current_status:
            logger.info('Switch changed from {} to {}'.format(
                self.tog_switch_status,
                current_status
            ))
            self.tog_switch_status = current_status
            self.__update_status()
        else:
            logger.debug('No changes.')

    def __update_status(self):
        user = self.twitter.GetUser(user_id=self.twitter_id)
        logger.debug('Twitter user {} current bio: {}'.format(
            str(self.twitter_id),
            user.description
        ))
        user_bio = user.description.split(
                                self.twitter_bio_message
                               )[0]
        updated_description = '{} {} {}'.format(
            user_bio,
            self.twitter_bio_message,
            self.space_states[self.tog_switch_status]
        )
        self.twitter.UpdateProfile(description=updated_description)
        logger.info('Twitter new bio: {}'.format(updated_description))

    def run(self):
        self.mqtt.on_connect = self.__on_connect
        self.mqtt.on_message = self.__on_message
        self.mqtt.loop_forever()


if __name__ == '__main__':
    try:
        tog = TOGKnifeSwitch({
            'twitter_consumer_key': env.get('TWITTER_CONSUMER_KEY', 'missing'),
            'twitter_consumer_secret': env.get('TWITTER_CONSUMER_SECRET', 'missing'),  # noqa: E501
            'twitter_access_token_key': env.get('TWITTER_ACCESS_TOKEN', 'missing'),  # noqa: E501
            'twitter_access_token_secret': env.get('TWITTER_ACCESS_TOKEN_SECRET', 'missing'),  # noqa: E501
            'twitter_id': env.get('TWITTER_ID', 76108817),
            'twitter_bio_message': env.get('TWITTER_BIO_MESSAGE', ' Space is: '),  # noqa: E501
            'mqtt_host': env.get('MQTT_HOST', '10.48.1.254'),
            'mqtt_port': env.get('MQTT_PORT', 1883),
            'mqtt_keepalive': env.get('MQTT_KEEPALIVE', 60),
            'mqtt_topic': env.get('MQTT_TOPIC', '/tog/sensors/knife_switch/knife_switch_ca750200'),  # noqa: E501
            'tog_switch_status': env.get('TOG_SWITCH_STATUS', None),
            'tog_open_status': env.get('TOG_OPEN_STATUS', 'OPEN'),
            'tog_closed_status': env.get('TOG_CLOSED_STATUS', 'CLOSED')
        })
        tog.run()
    except Exception as e:
        logger.critical('Oops: ', exc_info=True)
