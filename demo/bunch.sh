#!/bin/zsh

for i in {1..20}; do
  fname=$(printf '%02d\n' $i)
  node render.js
  mv demo.webm demo_${fname}.webm
done
