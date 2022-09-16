#!/usr/bin/env sh

echo "Copying required data..."

rm -rf ./Xonotic-clean
mkdir -p ./Xonotic-clean/data

cp ./Xonotic/xonotic-linux64-dedicated ./Xonotic-clean
cp ./Xonotic/data/xonotic-*-data.pk3 ./Xonotic-clean/data
cp ./Xonotic/data/xonotic-*-maps.pk3 ./Xonotic-clean/data
cp ./Xonotic/data/xonotic-*-nexcompat.pk3 ./Xonotic-clean/data

echo "Copied data!"
