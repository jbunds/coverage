#!/bin/zsh

# this script runs the render.js script 20 times to create 20 numbered demo WebM
# files as candidates for a good demo recording, because even after constraining
# mouse-pointer overshoot and non-centered clicks on HTML elements, GhostCursor
# will still sometimes move the mouse pointer outside of the browser's viewport,
# or make erratic and unnatural-looking mouse movements

for i in {1..20}; do
  fname=$(printf '%02d\n' $i)
  node render.js
  mv demo.webm demo_${fname}.webm
done
