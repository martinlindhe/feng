# feng TODO - May 2023


### USER FRIENDLINESS
- LOW: error if a struct name occurs more than once
- LOW: error if a layout name (label) occurs more than once

- error when no_magic is not a boolean, eg:

      no_magic:
          - offset: 0000
            match: 18 18 2a 1a


- syntax error like this is not caught, should be error:

    u8 Flags: ??
      bit b0000_0001: EndOfRecord
      bit b0000_0010: VideoSector


### USABILITY + POLISH

HI: rework magic matching (https://github.com/martinlindhe/feng/issues/2)

fix failing tests

simple cli hex navigator, similar to [formats](https://github.com/martinlindhe/formats/tree/master/cmd/formats) cli app



### YAML FORMAT

- "u1" (1 bit), "u4" (4 bits), "u12" (12 bits) data types:
  - rework internals to use golang bitreader. needed by archive/bzip2, image/bpg

- ability to extend a template with structs from another template. in order to reuse templates for commonly embedded formats such as RIFF, Exif, PNG
