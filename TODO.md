feng TODO - april 2021


- HI: allow complex "if" expression with multiple checks. allow "ELSE" block (needed for proper mapping of palette) (PCX)

- HI: figure out how to decode image data stream (between SOI and EOI markers) (JPEG)

- HI: "data_sub_block[] Image data: ??" with custom type and "data: eos" marker to end slice stream  (GIF)



# SLICE SYNTAX (done, need verify)
- HI: "chunk[] Chunk" syntax (PNG-DONE, JPEG-TODO VERIFY, GIF-TODO VERIFY)



# CUSTOM TYPES:
- HI: decode "PLTE" chunk (need to allow custom type "rgb")  (PNG)
- HI: allow custom type in struct for DDS_PIXELFORMAT (DDS)
- MID: need to use custom "rgb" type as defined (GIF)


# USER FRIENDLINESS
LOW: error if a struct name occurs more than once
LOW: error if a layout name (label) occurs more than once

LOW: error if field name is reserved, like "offset", "len", "FILE_SIZE", xxx

LOW-future-features: crc32 type (7z, gzip)



# TEMPLATE DECORATION
- MAX: template directive to append text to current struct Label, such as PNG/GIF/JPEG chunk name
- MID: template directive for "SAMPLE PLEASE!"
- MID: allow to append text to current section label with special directive
- MID: offer special template %INDEX% to decorate label!
