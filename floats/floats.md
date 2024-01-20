# Floats and money

Accurately representing financial transactions is essential for software developers. 
This is particularly true in the case of dealing with fiat currency amounts, as errors in representation can have significant consequences.

Working with fiat money in development often comes with complexities, and developers frequently opt for simpler 
solutions without much consideration, which leads to potential fatal errors. In this article, we will explore a common 
mistake made by developers when dealing with fiat currencies.

It is important to acknowledge that cryptocurrencies like Bitcoin and Ethereum have significantly distinct representation 
and storage mechanisms compared to fiat currencies. Consequently, this article will focus exclusively on fiat currencies and will not address cryptocurrencies.

# Float/Double precision
Developers often use float or double types for monetary values in their code, but this can lead to inaccuracies in 
financial calculations due to the way these types store and represent data. 
The multitude of float and double types values is characterized by a finite representation of decimals,
i.e. some values cannot be defined precisely.

This limitation arises due to the fact that floating-point numbers use a binary representation that works in base-2,
whereas the decimal system used to represent currency is a base-10 one. As a result, some decimal values, such as 0.1, 
cannot be represented exactly in a binary format. 
When converted to a floating-point number, the representation of 0.1 will be slightly different from its actual value, 
leading to small errors in calculations that use this value.

# IEEE 754
The IEEE 754 standard specifies how floating-point numbers, including both single-precision (float32) and 
double-precision (float64) types, are represented in binary form.

A floating-point number is composed of three parts:
- the sign bit
- the exponent
- the mantissa (sometimes called the significand)

The sign bit is a single bit that determines the sign of a number. It is set to `1` for negative numbers and `0` for positive numbers.

The exponent bit is a group of bits that determines the order of magnitude of a number. 
It represents a power of 2 that is added to the mantissa to get the final value. 
In the single-precision format, the exponent is an 8-bit value, and in the double-precision format, it is an 11-bit value.

The mantissa is a group of bits that represents the significant digits of a number.
It is the fraction part of a number and is multiplied by 2 raised to the power of the exponent to get the final value. 
In the single-precision format, the mantissa is a 23-bit value, and in the double-precision format, it is a 52-bit value.
Let's have a look at how 0.1 will be stored according to IEEE754.

# IEEE 754 number -> bytes
According to the standard, the following algorithm should be used:
- Convert the integer part to binary. In this case, the integer part is 0, so the binary representation of the integer part is 0.
- Multiply the fractional part by 2. In this case, 0.1 * 2 = 0.2.
- Take the integer part of the result and add it to the binary representation. In this case, the integer part of 0.2 is 0, 
  so the binary representation becomes 0.0.
- Repeat steps 2 and 3, each time multiplying the fractional part by 2. 
  In this case, the next step gives 0.4, so the binary representation becomes 0.00.
- Keep repeating these steps until either the fractional part becomes zero or the desired level of precision is achieved.

As a result, for `0.1`:
```
0.1 * 2 = 0.2 -> 0
0.2 * 2 = 0.4 -> 0
0.4 * 2 = 0.8 -> 0
0.8 * 2 = 1.6 -> 1
0.6 * 2 = 1.2 -> 1
0.2 * 2 = 0.4 -> 0
0.4 * 2 = 0.8 -> 0
0.8 * 2 = 1.6 -> 1
0.6 * 2 = 1.2 -> 1
0.2 * 2 = 0.4 -> 0
0.4 * 2 = 0.8 -> 0
0.8 * 2 = 1.6 -> 1
0.6 * 2 = 1.2 -> 1
0.2 * 2 = 0.4 -> 0
0.4 * 2 = 0.8 -> 0
0.8 * 2 = 1.6 -> 1
0.6 * 2 = 1.2 -> 1
...
```
The resulting binary representation of 0.1 is:
```
0.0001100110011001100110011001100110011001100110011001100110011001100…
```
Next, we need to convert a number to a scientific notation. To convert the binary representation of 0.1 to a scientific notation, we need to shift the decimal point until the leftmost bit is equal to 1. In this case we shift the decimal point 4 positions to the left:
1.100110011001100110011001100110011001100110011001100110011001100…b * 2^(-4)
To convert the binary scientific notation to the float32, we need to adjust the exponent and the mantissa to fit the bit size of the float32.
First, let me remind you the structure of a float32 number. It consists of:
1 bit for the sign (positive or negative)
8 bits for the exponent
23 bits for the mantissa

In the case of `0.1` representation:
- The sign bit is 0, since 0.1 is a positive number.
- Determine the exponent: we add the bias (`127`) to the exponent value (`-4`) and convert the result (`123`) to binary: `01111011`
  This is the 8-bit exponent value for our float32 number.
- Determine the mantissa: we take the first 23 bits of the normalized binary number 
  (including the implicit leading 1, which is not explicitly stored). In this case, we have:
  `10011001100110011001100`

Combine the sign, the exponent, and the mantissa into a single 32-bit value, with the sign bit in the most significant position, 
followed by the exponent bits, followed by the mantissa bits. The 32-bit representation of 0.1 in the float32 format is:
```
0 01111011 10011001100110011001100
```

# IEEE 754 bytes -> number
To convert a sequence of bytes to a decimal number, we need to follow the reverse procedure of the one we used to convert 
a number to bytes.

- First, we determine the sign of the number by looking at the leftmost bit, which is the most significant bit (MSB) and represents the sign. 
  In this case, the leftmost bit is 0, which means the number is positive.
- Next, we determine the exponent by looking at the following 8 bits, which make up the exponent field.
  The exponent is represented in biased notation, which means that the true exponent is obtained by subtracting a 
  bias value from the value of the exponent field. For the float32, the bias value is `127`.
  In this case, the exponent field is `01111011`, which corresponds to the decimal value `123`. To get the true exponent, 
  we subtract the bias value of `127`, getting the exponent `123–127 = -4`.
  `2^(−4) = 0.0625`
- The mantissa is represented by the remaining 23 bits. However, the mantissa is normalized, 
  which means that the first bit is always equals to 1 and is therefore not explicitly stored.
  So we add an implicit leading 1 bit to the mantissa, which gives us a binary value `1.10011001100110011001100`.
  To convert the mantissa to a decimal value, we need to sum the values of the bits after the implicit leading first bit. 
  Each bit represents a fractional power of 2, with the first bit after the leading 1 representing 1/2, 
  the second bit representing 1/4, the third bit representing 1/8, and so on.
  In this case, the mantissa value can be calculated as:
  ```
  1.10011001100110011001100 = 1 × 2⁰ + 1/2 × 2^-1 + 1/4 × 2^-2 + 1/8 × 2^-3 +… = 1.60000002384185791015625
  ```

And now we need to multiply these components:
```
1 × 0.0625 × 1.60000002384185791015625 = 0.100000001490116119384765625
```

# Conclusion
When we convert the binary representation back to decimal, we end up with a value that is very close to, but not exactly equal to 0.1. 
This is due to the fact that not all decimal values can be represented exactly in binary format, as we discussed earlier.
This can lead to accuracy issues when using floating-point types for financial calculations, that makes them unsuitable for financial calculations.
There are two ways to represent fiat currency amounts in programming:
- custom decimal types
- integers

Custom decimal types, such as the BigDecimal type in Java, offer the advantage of being able to represent fractions of a penny or cent.
There are two problems with these types: performance (the calculations are very slow) and the necessity to always apply 2-digit precision.

The best way to represent fiat currency amounts is Integers (int64/uint64). They are faster to perform calculations
and they use less memory compared to decimals. Moreover, using integers guarantees that the values consistently 
represent physical amounts of money, eliminating the risk of rounding errors and other inaccuracies.

However, when using integer data types, it's important to remember to always represent amounts in the smallest denomination 
of the currency being used. For example, when working with US dollars, amounts should be represented in cents, not dollars.