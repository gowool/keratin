package middleware

// Copied from https://github.com/labstack/echo to avoid nuances around the specific
//
// -------------------------------------------------------------------
// The MIT License (MIT)
//
// Copyright (c) 2022 LabStack
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
// -------------------------------------------------------------------

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"net/url"
	"sync"
)

func createRandomStringGenerator(length uint8) func() string {
	return func() string {
		return randomString(length)
	}
}

// https://tip.golang.org/doc/go1.19#:~:text=Read%20no%20longer%20buffers%20random%20data%20obtained%20from%20the%20operating%20system%20between%20calls
var randomReaderPool = sync.Pool{New: func() any {
	return bufio.NewReader(rand.Reader)
}}

const randomStringCharset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const randomStringCharsetLen = 52 // len(randomStringCharset)
const randomStringMaxByte = 255 - (256 % randomStringCharsetLen)

func randomString(length uint8) string {
	reader := randomReaderPool.Get().(*bufio.Reader)
	defer randomReaderPool.Put(reader)

	b := make([]byte, length)
	r := make([]byte, length+(length/4)) // perf: avoid read from rand.Reader many times
	var i uint8 = 0

	// security note:
	// we can't just simply do b[i]=randomStringCharset[rb%len(randomStringCharset)],
	// len(len(randomStringCharset)) is 52, and rb is [0, 255], 256 = 52 * 4 + 48.
	// make the first 48 characters more possibly to be generated then others.
	// So we have to skip bytes when rb > randomStringMaxByt

	for {
		_, err := io.ReadFull(reader, r)
		if err != nil {
			panic("unexpected error happened when reading from bufio.NewReader(crypto/rand.Reader)")
		}
		for _, rb := range r {
			if rb > randomStringMaxByte {
				// Skip this number to avoid bias.
				continue
			}
			b[i] = randomStringCharset[rb%randomStringCharsetLen]
			i++
			if i == length {
				return string(b)
			}
		}
	}
}

func validateOrigins(origins []string, what string) error {
	for _, o := range origins {
		if err := validateOrigin(o, what); err != nil {
			return err
		}
	}
	return nil
}

func validateOrigin(origin string, what string) error {
	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("can not parse %s: %w", what, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s is missing scheme or host: %s", what, origin)
	}
	if u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%s can not have path, query, and fragments: %s", what, origin)
	}
	return nil
}
