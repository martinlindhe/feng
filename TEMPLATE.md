# directives


    data: invalid       invalidates the file
    data: eos           marks end of stream (for slices)    TODO

    endian: big         big/little. set endian


# constants

    FILE_SIZE           the file size in bytes

    self.offset         current offset                  TODO


# data types
    u8, u16, u32, u64
    ascii[5]
    asciiz              zero terminated ascii string
    time_t_32           32-bit unix timestamp


# arrays

    u32[4]
    u8[FILE_SIZE-10]


# tricks

    u8[FILE_SIZE-self.offset] Extra: ??             tags the remaining bytes
