# feng yaml file format template specification

VERSION 0 - DRAFT. JULY 2022


# Directives

```yaml
data: invalid                           # invalidates the file
data: unseen                            # marks data as previously unseen, asking the user to submit a sample at the end of parsing

endian: big                             # set endian to big/little

label: '"DIRENTRY"'                     # set label decoration for the current struct
label: self.Key + " = " self.Value      # evaluate strings

offset: self.BaseOffset                 # set offset to evaluated struct field

parse: stop                             # stops parsing. used to signal custom end-of-stream conditions
```

# Endianness

In order to parse multi-byte data fields, the template needs to know the byte order (big/little).

If endianness is unknown by the reader, it will error out.

You can set endian in the top level document:

```yaml
endian: little
kind: archive
...
```

Or on a single field:

```yaml
be:filetime   Time: ??
```

You can also change endianness during struct evaluation, like this:

```yaml
endian: big
u16 Big A: ??
u16 Big B: ??
endian: little
u16 Little A: ??
```

A common pattern is:

```yaml
endian: big    # top level default endianness
...
structs:
  segment:
    ...
    # in a struct, change endianness
    u16 Align:
      eq c'MM': BIG_ENDIAN
      eq c'II': LITTLE_ENDIAN
    if self.Align == LITTLE_ENDIAN:
      endian: little
```



# Pre-defined values

```yaml
FILE_SIZE           # the file size in bytes
OFFSET              # current offset
self                # evaluates to the current struct
self.index          # slice-based iteration index, 0-based
```


# Required byte sequences

You can specify a required byte sequence like this
```yaml
ascii[2] Magic:    c'PK'

u16 TYPE: 00 01 ff
```

Hex byte strings is always expressed in network byte order


# Built-in functions

```
abs(-95)       = 95     returns absolute value
peek_i16(123)  = -1     returns i16 value from offset
peek_i16("0100")        hex string offset
peek_i32(123)  = -1     returns i32 value from offset
atoi("123")    = 123    returns integer from alphanumeric string
otoi("123")    = 83     returns integer from octal numeric string (archives/tar)
alignment(3,4) = 1      returns the number of bytes needed to align the first arg to the second arg (add padding bytes)
not(self.Value, 4, 5) = true   returns true if self.Value is neither 4 or 5
either(self.Value, 4, 5) = false   returns true if self.Value is either 4 or 5
```

# Data types

numeric

    u8, u16, u32, u64, f32


numeric bit fields

```yaml
u16 Type:
  eq 0000: TYPE_NULL
  eq 0001: TYPE_STRING_POOL
  default: invalid
```

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
    dostimedate         32-bit MS-DOS (dostime, dosdate)

colors

    rgb8                3 byte values for R, G, B

3d data

    xyzm32              x,y,z,m matrix of f32 values

data tagging (for extraction feature)

    raw:u8[40]                      mark area as raw data (extracted as-is)

    u32 Size: ??
    compressed:zlib[self.Size]      mark area as zlib compressed data
    compressed:lzo1x[self.Size]     mark area as LZO1x-compatible data
    compressed:lz4[self.Size]       mark area as LZ4-compressed data
    compressed:deflate[self.Size]   mark area as DEFLATE compressed data

    filename: self.Filename         set the filename to use while extracting for the next data area



variable length encoding

    vu32                        variable-length u32 (fonts/woff2, images/bpg)
    vu64                        variable-length u32 (archives/xz, archives/7zip)

pattern matching data types

  until: u8 scanData ff d9            maps all bytes to scanData until marker is seen (images/jpeg)


# Evaluate string keys to labels

```yaml
label: >
  "FILE_OR_DIR " + self.FileName

label: self.FileName + " (FILE_OR_DIR)"
```


# Constants

The `eq` and `bit` pattern matches automatically evaluates to constants

eq:
```yaml
u16 Type:
  eq 0000: TYPE_NULL
  eq 0001: TYPE_STRING_POOL
  default: invalid
if self.Type == TYPE_NULL:
  u8 Footer: ??
```

bit:
```yaml
u16 Flag:
  bit b0000_0000_0111_1111: Lo
  bit b0000_1111_1000_0000: B3
  bit b1111_0000_0000_0000: Hi
if self.Flag & Lo:
  u8 LoData: ??
```

# Arrays

    u32[4]
    u8[FILE_SIZE-10]




# Slices

    chunk[]


# Tricks

    u8[FILE_SIZE - OFFSET] Extra: ??         tags the remaining bytes


# If-statements

NOTE: variables used in if-statements cannot contain spaces

```yaml
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

if either(self.HeaderSize, V3, V4, V5):
  i32 Width: ??


# example from cab.yml
u16 Flags:
  bit b00000000_00000100: ReservePresent  # ReservePresent automatically becomes a constant

if self.Flags & ReservePresent:
  u16 cbCFHeader: ??
```
