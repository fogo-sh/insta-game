#!/usr/bin/env sh

echo "Copying required data..."

rm -rf ./Xonotic-clean
mkdir -p ./Xonotic-clean/data

cp ./Xonotic/xonotic-linux64-dedicated ./Xonotic-clean
cp ./Xonotic/data/xonotic-20170401-data.pk3 ./Xonotic-clean/data
cp ./Xonotic/data/xonotic-20170401-maps.pk3 ./Xonotic-clean/data
cp ./Xonotic/data/xonotic-20170401-nexcompat.pk3 ./Xonotic-clean/data

echo "Copied data!"
