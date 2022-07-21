# feng yaml file format template specification

VERSION 0 - DRAFT. JULY 2022


# directives

```
data: invalid                           invalidates the file
data: unseen                            marks data as previously unseen, asking the user to submit a sample at the end of parsing

endian: big                             set endian to big/little

label: '"DIRENTRY"'                     set label decoration for the current struct
label: self.Key + " = " self.Value      evaluate strings

offset: self.BaseOffset                 set offset to evaluated struct field

parse: stop                             stops parsing. used to signal custom end-of-stream conditions
```

# endianness

on a single field:

```
be:filetime   Time: ??
```

until another directive:

```
endian: big
u16 Big A: ??
u16 Big B: ??
endian: little
u16 Little A: ??
```


# pre-defined values

```
FILE_SIZE           the file size in bytes
OFFSET              current offset
field.len           field length  # XXX BROKEN/UNUSED
self                evaluates to the current struct
self.index          slice-based iteration index, 0-based
```


# required byte sequences

You can specify a required byte sequence like this
```
ascii[2] Magic:    c'PK'

u16 TYPE: 00 01 ff
```

Hex byte strings is always expressed in network byte order


# built-in functions


abs(-95)       = 95  returns absolute value
peek_i16(123)  = -1  returns i16 value from offset
peek_i16("0100")    hex string offset
peek_i32(123)  = -1  returns i32 value from offset
atoi("123")    = 123   returns integer from alphanumeric string
otoi("123")    = 83    returns integer from octal numeric string (archives/tar)
alignment(3,4) = 1     returns the number of bytes needed to align the first arg to the second arg
not(self.Value, 4, 5) = true   returns true if self.Value is neither 4 or 5
either(self.Value, 4, 5) = false   returns true if self.Value is either 4 or 5


# data types

numeric

    u8, u16, u32, u64


numeric bit fields

    u16 Type:
      eq 0000: TYPE_NULL            these types will evaluate as constants
      eq 0001: TYPE_STRING_POOL
      default: invalid


text

    ascii[5]            ascii string
    asciiz              zero terminated ascii string
    utf16[5]            utf16 string    (utf16 le == wchar_t)
    utf16z              zero terminated utf16 string


date / time

    time_t_32           32-bit unix timestamp, in UTC
    filetime            64-bit windows timestamp, in UTC
    dosdate             16-bit MS-DOS datestamp, in UTC
    dostime             16-bit MS-DOS timestamp, in UTC

colors
    rgb8                3 byte values for R, G, B


data (for extraction feature)

    raw:u8[size]                mark area as file data
    compressed:zlib[self.Size]  mark area as zlib compressed data
    compressed:lz4[self.Size]   mark area as lz4-compressed data
    compressed:deflate[self.Size] mark area as DEFLATE compressed data


variable length encoding

    vu32                        variable-length u32 (fonts/woff2)
    vu64                        variable-length u32 (archives/xz, archives/7zip)

# pattern matching data types

  until: u8 scanData ff d9            maps all bytes to scanData until marker is seen

used by images/jpeg



# arrays

    u32[4]
    u8[FILE_SIZE-10]




# slices

    chunk[]


# tricks

    u8[FILE_SIZE-self.offset] Extra: ??         tags the remaining bytes


# if-statements

NOTE: variables used in if-statements cannot contain spaces

```
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
  u16 cbCFHeader: ??
```
