using System;

public class Test {
    public static void Main(string[] args) {
        Console.WriteLine("Hello World!");

        // XXX decompress data

        var compressed = new byte[] {
            0x04, 0x00, 0x00, 0x00, 0x1F, 0xFF, 0xFE, 0x3C, 0x00, 0x3F, 0x00, 0x78, 0x00, 0x6D, 0x00, 0x6C,
            0x00, 0x20, 0x00, 0x76, 0x00, 0x65, 0x00, 0x72, 0x00, 0x73, 0x00, 0x69, 0x00, 0x6F, 0x00, 0x6E,
            0x00, 0x3D, 0x00, 0x22, 0x00, 0x04, 0x31, 0x00, 0x2E, 0x00, 0x30, 0x20, 0x07, 0x00};

        var uncompressed = new byte[10000000];

        var len = Decompress(compressed, uncompressed);

        //Console.WriteLine("Decoded %d bytes", len);
    }

    public static int Decompress(byte[] input, byte[] output)
    {
        int i = 0;
        int o = 0;

        int inputLength = input.Length;
        int outputLength = output.Length;

        while (i < inputLength)
        {
            uint control = input[i++];

            if (control < (1 << 5))
            {
                int length = (int)(control + 1);

                if (o + length > outputLength)
                {
                    Console.WriteLine("Invalid1");
                    throw new InvalidOperationException();
                }

                Array.Copy(input, i, output, o, length);
                i += length;
                o += length;
            }
            else
            {
                int length = (int)(control >> 5);
                int offset = (int)((control & 0x1F) << 8);

                if (length == 7)
                {
                    length += input[i++];
                }
                length += 2;

                offset |= input[i++];

                if (o + length > outputLength)
                {
                    Console.WriteLine("Invalid2");
                    throw new InvalidOperationException();
                }

                offset = o - 1 - offset;
                if (offset < 0)
                {
                    Console.WriteLine($"InvalidOperation3 at {i}: o={o}, offset={offset}");
                    throw new InvalidOperationException();
                }

                int block = Math.Min(length, o - offset);
                Array.Copy(output, offset, output, o, block);
                o += block;
                offset += block;
                length -= block;

                while (length > 0)
                {
                    output[o++] = output[offset++];
                    length--;
                }
            }
        }

        return o;
    }
}
