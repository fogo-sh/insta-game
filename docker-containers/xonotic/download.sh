#!/usr/bin/env sh

FILE=xonotic.zip
SHA_512_SUM=cb39879e96f19abb2877588c2d50c5d3e64dd68153bec3dd1bebedf4d765e506afa419c28381d7005aed664cb1a042571c132b5b319e4308cab67745d996c2a6

validate() {
  echo "$SHA_512_SUM $FILE" | sha512sum -c
}

exists() {
  echo "$FILE exists, validating checksum"
  validate
}

if [ -f "$FILE" ]; then
  exists
  unzip -o "$FILE"
else
  echo "$FILE does not exist, downloading..."
  curl https://dl.xonotic.org/xonotic-0.8.6.zip -L -o "$FILE"
  exists
  unzip -o "$FILE"
fi
