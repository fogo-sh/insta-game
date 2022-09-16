#!/usr/bin/env sh

FILE=xonotic.zip
SHA_512_SUM=4ffc4b73eeb5f580d178a98419d5b44cbff0c56e356a62baa729b5b7a6c3d43b2b425b123428c9b1bf3f4718eaf61bcf5d62914521cc061c7563a253440c807e

validate() {
  echo "$SHA_512_SUM $FILE" | sha512sum -c
}

exists() {
  echo "$FILE exists, validating checksum"
  validate
}

if [ -f "$FILE" ]; then
  unzip "$FILE"
  exists
else 
  echo "$FILE does not exist, downloading..."
  curl https://dl.xonotic.org/xonotic-0.8.5.zip -L -o "$FILE"
  unzip "$FILE"
  exists
fi
