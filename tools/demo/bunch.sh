#!/usr/bin/env bash

# this script runs the render.js script 20 times to create 20 numbered demo MP4
# files as candidates for a good demo recording, because even after constraining
# mouse-pointer overshoot and non-centered clicks on HTML elements, GhostCursor
# will still sometimes move the mouse pointer outside of the browser's viewport,
# or make erratic and unnatural-looking mouse movements

dir=$(dirname "${BASH_SOURCE[0]}")

for i in {1..20}; do
  node "$dir/render.ts"
  mv demo.mp4 demo_$(printf '%02d' $i).mp4
done
