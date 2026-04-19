#!/bin/zsh

# this script runs the render.js script 20 times to create 20 numbered
# demo WebM files as candidates for a good demo recording

for i in {1..20}; do
  fname=$(printf '%02d\n' $i)
  node render.js
  mv demo.webm demo_${fname}.webm
done
