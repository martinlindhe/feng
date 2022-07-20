# feng TODO - July 2022



# USER FRIENDLINESS
- LOW: error if a struct name occurs more than once
- LOW: error if a layout name (label) occurs more than once
- LOW: error if field name is reserved, like "offset", "len", "FILE_SIZE"







# MATCHING

- first match on magic file numbers. if no match, try the formats without those in classic full format sense...



### USABILITY + POLISH

for no_magic formats: match on input file extension

feng printout: show complete printout of all hex values in long arrays with an cli option (default to show only 1st)

logging: use something better

fix failing tests

yaml format: mark up certain conditions for "want sample please" in a standardized way

simple cli hex navigator, similar to formats cli app

ability to parse and use additional templates in a user folder

LATER yaml format: single bits: "u1" (1 bit), "u24" (24 bits), "u4" (4 bits) data types:
  - rework internals to use golang bitreader by default so we can implement support for bzip2 bit stream?

