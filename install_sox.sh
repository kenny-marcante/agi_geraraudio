#! /bin/bash

echo "system update"
apt-get --quiet update


echo "installing SOX"
apt-get install --quiet sox libsox-fmt-mp3 libsox-fmt-alsa libsox-fmt-base libsox3 libsoxr0 --yes


