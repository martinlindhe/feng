# feng TODO - July 2021


# CUSTOM TYPES:
- MAX: decode "PLTE" chunk (need to allow custom type "rgb")  (PNG)
- MAX: allow custom type in struct for DDS_PIXELFORMAT (DDS)
- MID: need to use custom "rgb" type as defined (GIF)



# USER FRIENDLINESS
- LOW: error if a struct name occurs more than once
- LOW: error if a layout name (label) occurs more than once
- LOW: error if field name is reserved, like "offset", "len", "FILE_SIZE"


# TEMPLATE DECORATION
- MAX: template directive to append text to current struct Label, such as PNG/GIF/JPEG chunk name
- MID: template directive for "SAMPLE PLEASE!"
- MID: allow to append text to current section label with special directive
- MID: offer special template %INDEX% to decorate label!




# MATCHING

- first match on magic file numbers. if no match, try the formats without those in classic full format sense...





### USABILITY + POLISH

for no_magic formats: match on input file extension

feng printout: show complete printout of all hex values in long arrays with an cli option (default to show only 1st)

logging: use something better

fix failing tests

rework internals to use golang bitreader by default so we can implement support for bzip2 bit stream?

