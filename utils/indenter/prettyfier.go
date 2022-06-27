package indenter

import (
	"fmt"
	"strings"
)

type indenter struct{}

func Indenter() indenter {
	return indenter{}
}

var _buffer string

func indent() string {
	return strings.Repeat("  ", _level)
}

func (indenter) Start(str string) indenter {
	_buffer = str
	return Indenter()
}

type stringableString string
func (s stringableString) String() string {
	return string(s)
}

func (i indenter) NestStrings(strs ...string) indenter {
	return i.NestStringsSep("", strs...)
}

func (i indenter) NestStringsSep(sep string, strs ...string) indenter {
	stringers := make([]fmt.Stringer, len(strs))
	for i, v := range strs {
		stringers[i] = stringableString(v)
	}
	return i.NestSep(sep, stringers...)
}

func (indenter) Nest(strs ...fmt.Stringer) indenter {
	return Indenter().NestSep("", strs...)
}

func (indenter) NestSep(sep string, strs ...fmt.Stringer) indenter {
	if len(strs) == 1 {
		buf := _buffer
		buf += strs[0].String()
		_buffer = buf

		return Indenter()
	}

	_level++
	for i, str := range strs {
		buf := _buffer
		buf += "\n" + indent() + str.String()
		if i < len(strs)-1 {
			buf += sep
		}
		_buffer = buf
	}
	_level--
	_buffer += "\n"
	return Indenter()
}

func (indenter) Append(str string) string {
	buf := _buffer
	res := indent() + str
	_buffer = buf
	return res
}

func (indenter) NestThunked(strs ...func() string) indenter {
	return Indenter().NestThunkedSep("", strs...)
}

func (indenter) NestThunkedSep(sep string, strs ...func() string) indenter {
	if len(strs) == 1 {
		buf := _buffer
		buf += strs[0]()
		_buffer = buf

		return Indenter()
	}

	_level++
	for i, str := range strs {
		buf := _buffer
		buf += "\n" + indent() + str()
		if i < len(strs)-1 {
			buf += sep
		}
		_buffer = buf
	}
	_level--
	_buffer += "\n"
	return Indenter()
}

func (indenter) NestThunkedPresep(sep string, strs ...func() string) indenter {
	if len(strs) == 1 {
		buf := _buffer
		buf += strs[0]()
		_buffer = buf

		return Indenter()
	}

	_level++
	for i, str := range strs {
		buf := _buffer
		if i != 0 {
			buf += "\n"
			buf += indent()
			buf += sep
		} else {
			buf += "  "
		}
		buf += str()
		_buffer = buf
	}
	_level--
	_buffer += "\n"
	return Indenter()
}

func (indenter) End(str string) string {
	var res string
	if _buffer[len(_buffer)-1] == '\n' {
		res = _buffer + indent() + str
	} else {
		res = _buffer + str
	}
	_buffer = ""
	return res
}

var _level = 0
