# feng TODO - May 2023


# USER FRIENDLINESS
- LOW: error if a struct name occurs more than once
- LOW: error if a layout name (label) occurs more than once



### USABILITY + POLISH

HI: rework magic matching (https://github.com/martinlindhe/feng/issues/2)

HI: ability to parse and use additional templates in a user folder

fix failing tests

simple cli hex navigator, similar to [formats](https://github.com/martinlindhe/formats/tree/master/cmd/formats) cli app



### YAML FORMAT

- single bits: "u1" (1 bit), "u4" (4 bits) data types:
  - rework internals to use golang bitreader by default. needed by archive/bzip2, image/bpg

- ability to extend a template with structs from another template. in order to reuse templates for commonly embedded formats such as RIFF, Exif, PNG
