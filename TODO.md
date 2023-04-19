# feng TODO - July 2022



# USER FRIENDLINESS
- LOW: error if a struct name occurs more than once
- LOW: error if a layout name (label) occurs more than once
- LOW: error if field name is reserved, like "OFFSET", "FILE_SIZE"




# MATCHING

- first match on magic file numbers. if no match, try the formats without those in classic full format sense...



### USABILITY + POLISH

HI: ability to parse and use additional templates in a user folder

feng printout: show complete printout of all hex values in long arrays with an cli option (default to show only 1st)

logging: use something better

fix failing tests

simple cli hex navigator, similar to [formats](https://github.com/martinlindhe/formats/tree/master/cmd/formats) cli app



yaml format: single bits: "u1" (1 bit), "u24" (24 bits), "u4" (4 bits) data types:
  - rework internals to use golang bitreader by default. needed by archive/bzip2, image/bpg

yaml format: ability to extend a template with structs from another template. in order to reuse templates for commonly embedded formats such as RIFF, Exif, PNG
