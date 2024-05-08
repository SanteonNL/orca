# Rendering draw.io diagrams

Run the following `docker run` command to render SVG files for the draw.io/diagrams.net source files.
```shell
docker run -v ./:/data rlespinasse/drawio-export -f svg
mv export/*.svg .
```
