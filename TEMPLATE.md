# directives

    data: invalid       invalidates the file

    endian: big         big/little. set endian

    label: "APP0"       set label decoration for the current struct

    offset: self.BaseOffset       set offset to evaluated data field

    parse: stop          stops parsing. useful for custom end-of-stream conditions (CURRENTLY NOT NEEDED. MAY BE REMOVED)


# pre-defined values

    FILE_SIZE           the file size in bytes

    field.offset        field offset
    field.len           field length

    self.index        slice-based iteration index, 0-based


# constants

    ascii[2] BIG:    c'MM'
    ascii[2] LITTLE: c'II'

    u16 RES_STRING_POOL_TYPE: 00 01

Constants is always expressed in network byte order


# data types

    u8, u16, u32, u64
    ascii[5]            ascii string
    asciiz              zero terminated ascii string
    utf16[5]            utf16 string    (utf16 le == wchar_t)
    time_t_32           32-bit unix timestamp, in UTC
    filetime            64-bit windows timestamp, in UTC
    dosdate             16-bit MS-DOS datestamp, in UTC
    dostime             16-bit MS-DOS timestamp, in UTC


    raw:u8[size]                mark area as file data (for extraction feature)
    compressed:zlib[self.Size]  mark area as zlib compressed data (for extraction feature)

    u16 Type:
      eq 0000: TYPE_NULL            these types will evaluate as constants
      eq 0001: TYPE_STRING_POOL
      default: invalid


# arrays

    u32[4]
    u8[FILE_SIZE-10]

    u8[self.Data offset:self.Data size]         "start:length" offset syntax      used by images/ico.yml


# slices

    chunk[]


# tricks

    u8[FILE_SIZE-self.offset] Extra: ??         tags the remaining bytes


# if-statements

NOTE: variables used in if-statements cannot contain spaces

    if self.Signature == BIG:   # where big is a constant or a eq pattern type value
      ...

    if self.Signature == 5:
      ...

    # example from bmp.yml
    u32 HeaderSize:
      eq 0000_000c: V2   # V2 automatically becomes a constant
      eq 0000_0028: V3
      eq 0000_006c: V4
      eq 0000_007c: V5
      default: invalid

    if self.HeaderSize in {V3, V4, V5}:
      i32 Width: ??


    # example from cab.yml
    u16 Flags:
      bit b00000000_00000100: ReservePresent  # ReservePresent automatically becomes a constant

    if self.Flags & ReservePresent:
      u16 cbCFHeader: ??  # size of per-cabinet reserved area


# loops (TODO)

TODO consider lz4, where there is N data blocks and the last one has DataSize == 0.

    u32 DataSize:
      bit b0111_1111_11111111_11111111_11111111: DataSize
      bit b1000_0000_00000000_00000000_00000000: Uncompressed
    u8[self.DataSize.DataSize] Data: ??

How to parse this?
