# Rendering draw.io diagrams

Run the following `docker run` command to render SVG files for the draw.io/diagrams.net source files.
```shell
docker run -v ./:/data rlespinasse/drawio-export -f svg
mv export/*.svg .
```

# Rendering PlantUML diagrams
Run using docker with the following command:
```shell
docker run --rm -v $(pwd):/data plantuml/plantuml plantuml -o ./images/ -tsvg ./docs/*.puml
```

Or natively with the following command:
```shell
plantuml -o ./images/ -tsvg ./docs/*.puml
```
