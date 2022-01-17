# directives

    data: invalid       invalidates the file

    endian: big         big/little. set endian

    label: "APP0"       set label decoration for the current struct

    offset: self.BaseOffset       set offset to evaluated data field


# pre-defined values

    FILE_SIZE           the file size in bytes

    field.offset        field offset
    field.len           field length

    self.index        slice-based iteration index, 0-based


# constants

    ascii[2] BIG:    c'MM'
    ascii[2] LITTLE: c'II'


# data types

    u8, u16, u32, u64
    ascii[5]            ascii string
    asciiz              zero terminated ascii string
    utf16[5]            utf16 string    (utf16 le == wchar_t)
    time_t_32           32-bit unix timestamp, in UTC
    filetime            64-bit windows timestamp, in UTC
    dosdate             16-bit MS-DOS datestamp, in UTC
    dostime             16-bit MS-DOS timestamp, in UTC


# arrays

    u32[4]
    u8[FILE_SIZE-10]

    u8[self.Data offset:self.Data size]         "start:length" offset syntax      used by images/ico.yml


# slices

    chunk[]


# tricks

    u8[FILE_SIZE-self.offset] Extra: ??         tags the remaining bytes


# if-blocks

    if self.Signature in (BIG):
      ...

    if self.Signature in (LITTLE):
      ...

    if self.Signature notin (BIG, LITTLE):
      ...

    u8 Bit field:
      bit b1000_0000: High bit

    if self.Bit field.High bit in (1):   # true if bitfield value is exactly 1
      ...

    if self.Bit field.High bit:          # true if bitfield value is non-zero
      ...
