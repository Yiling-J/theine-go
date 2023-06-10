package main

// Code generated by github.com/tinylib/msgp DO NOT EDIT.

import (
	"github.com/tinylib/msgp/msgp"
)

// DecodeMsg implements msgp.Decodable
func (z *Bar) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Name":
			z.Name, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Number":
			z.Number, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Number")
				return
			}
		case "Float":
			z.Float, err = dc.ReadFloat32()
			if err != nil {
				err = msgp.WrapError(err, "Float")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z Bar) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 3
	// write "Name"
	err = en.Append(0x83, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Name)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	// write "Number"
	err = en.Append(0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Number)
	if err != nil {
		err = msgp.WrapError(err, "Number")
		return
	}
	// write "Float"
	err = en.Append(0xa5, 0x46, 0x6c, 0x6f, 0x61, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat32(z.Float)
	if err != nil {
		err = msgp.WrapError(err, "Float")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z Bar) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 3
	// string "Name"
	o = append(o, 0x83, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Number"
	o = append(o, 0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	o = msgp.AppendInt(o, z.Number)
	// string "Float"
	o = append(o, 0xa5, 0x46, 0x6c, 0x6f, 0x61, 0x74)
	o = msgp.AppendFloat32(o, z.Float)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Bar) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Number":
			z.Number, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Number")
				return
			}
		case "Float":
			z.Float, bts, err = msgp.ReadFloat32Bytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Float")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z Bar) Msgsize() (s int) {
	s = 1 + 5 + msgp.StringPrefixSize + len(z.Name) + 7 + msgp.IntSize + 6 + msgp.Float32Size
	return
}

// DecodeMsg implements msgp.Decodable
func (z *Foo) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			z.ID, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "ID")
				return
			}
		case "Str":
			z.Str, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Str")
				return
			}
		case "Int":
			z.Int, err = dc.ReadInt()
			if err != nil {
				err = msgp.WrapError(err, "Int")
				return
			}
		case "Pointer":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					err = msgp.WrapError(err, "Pointer")
					return
				}
				z.Pointer = nil
			} else {
				if z.Pointer == nil {
					z.Pointer = new(int)
				}
				*z.Pointer, err = dc.ReadInt()
				if err != nil {
					err = msgp.WrapError(err, "Pointer")
					return
				}
			}
		case "Name":
			z.Name, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Sentence":
			z.Sentence, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Sentence")
				return
			}
		case "RandStr":
			z.RandStr, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "RandStr")
				return
			}
		case "Number":
			z.Number, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Number")
				return
			}
		case "Regex":
			z.Regex, err = dc.ReadString()
			if err != nil {
				err = msgp.WrapError(err, "Regex")
				return
			}
		case "Map":
			var zb0002 uint32
			zb0002, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Map")
				return
			}
			if z.Map == nil {
				z.Map = make(map[string]int, zb0002)
			} else if len(z.Map) > 0 {
				for key := range z.Map {
					delete(z.Map, key)
				}
			}
			for zb0002 > 0 {
				zb0002--
				var za0001 string
				var za0002 int
				za0001, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Map")
					return
				}
				za0002, err = dc.ReadInt()
				if err != nil {
					err = msgp.WrapError(err, "Map", za0001)
					return
				}
				z.Map[za0001] = za0002
			}
		case "Array":
			var zb0003 uint32
			zb0003, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "Array")
				return
			}
			if cap(z.Array) >= int(zb0003) {
				z.Array = (z.Array)[:zb0003]
			} else {
				z.Array = make([]string, zb0003)
			}
			for za0003 := range z.Array {
				z.Array[za0003], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Array", za0003)
					return
				}
			}
		case "ArrayRange":
			var zb0004 uint32
			zb0004, err = dc.ReadArrayHeader()
			if err != nil {
				err = msgp.WrapError(err, "ArrayRange")
				return
			}
			if cap(z.ArrayRange) >= int(zb0004) {
				z.ArrayRange = (z.ArrayRange)[:zb0004]
			} else {
				z.ArrayRange = make([]string, zb0004)
			}
			for za0004 := range z.ArrayRange {
				z.ArrayRange[za0004], err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "ArrayRange", za0004)
					return
				}
			}
		case "Bar":
			var zb0005 uint32
			zb0005, err = dc.ReadMapHeader()
			if err != nil {
				err = msgp.WrapError(err, "Bar")
				return
			}
			for zb0005 > 0 {
				zb0005--
				field, err = dc.ReadMapKeyPtr()
				if err != nil {
					err = msgp.WrapError(err, "Bar")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Name":
					z.Bar.Name, err = dc.ReadString()
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Name")
						return
					}
				case "Number":
					z.Bar.Number, err = dc.ReadInt()
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Number")
						return
					}
				case "Float":
					z.Bar.Float, err = dc.ReadFloat32()
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Float")
						return
					}
				default:
					err = dc.Skip()
					if err != nil {
						err = msgp.WrapError(err, "Bar")
						return
					}
				}
			}
		case "Skip":
			if dc.IsNil() {
				err = dc.ReadNil()
				if err != nil {
					err = msgp.WrapError(err, "Skip")
					return
				}
				z.Skip = nil
			} else {
				if z.Skip == nil {
					z.Skip = new(string)
				}
				*z.Skip, err = dc.ReadString()
				if err != nil {
					err = msgp.WrapError(err, "Skip")
					return
				}
			}
		case "Created":
			z.Created, err = dc.ReadTime()
			if err != nil {
				err = msgp.WrapError(err, "Created")
				return
			}
		case "CreatedFormat":
			z.CreatedFormat, err = dc.ReadTime()
			if err != nil {
				err = msgp.WrapError(err, "CreatedFormat")
				return
			}
		case "ByteData":
			z.ByteData, err = dc.ReadBytes(z.ByteData)
			if err != nil {
				err = msgp.WrapError(err, "ByteData")
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *Foo) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 17
	// write "ID"
	err = en.Append(0xde, 0x0, 0x11, 0xa2, 0x49, 0x44)
	if err != nil {
		return
	}
	err = en.WriteInt(z.ID)
	if err != nil {
		err = msgp.WrapError(err, "ID")
		return
	}
	// write "Str"
	err = en.Append(0xa3, 0x53, 0x74, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.Str)
	if err != nil {
		err = msgp.WrapError(err, "Str")
		return
	}
	// write "Int"
	err = en.Append(0xa3, 0x49, 0x6e, 0x74)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Int)
	if err != nil {
		err = msgp.WrapError(err, "Int")
		return
	}
	// write "Pointer"
	err = en.Append(0xa7, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x65, 0x72)
	if err != nil {
		return
	}
	if z.Pointer == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = en.WriteInt(*z.Pointer)
		if err != nil {
			err = msgp.WrapError(err, "Pointer")
			return
		}
	}
	// write "Name"
	err = en.Append(0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Name)
	if err != nil {
		err = msgp.WrapError(err, "Name")
		return
	}
	// write "Sentence"
	err = en.Append(0xa8, 0x53, 0x65, 0x6e, 0x74, 0x65, 0x6e, 0x63, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Sentence)
	if err != nil {
		err = msgp.WrapError(err, "Sentence")
		return
	}
	// write "RandStr"
	err = en.Append(0xa7, 0x52, 0x61, 0x6e, 0x64, 0x53, 0x74, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.RandStr)
	if err != nil {
		err = msgp.WrapError(err, "RandStr")
		return
	}
	// write "Number"
	err = en.Append(0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	if err != nil {
		return
	}
	err = en.WriteString(z.Number)
	if err != nil {
		err = msgp.WrapError(err, "Number")
		return
	}
	// write "Regex"
	err = en.Append(0xa5, 0x52, 0x65, 0x67, 0x65, 0x78)
	if err != nil {
		return
	}
	err = en.WriteString(z.Regex)
	if err != nil {
		err = msgp.WrapError(err, "Regex")
		return
	}
	// write "Map"
	err = en.Append(0xa3, 0x4d, 0x61, 0x70)
	if err != nil {
		return
	}
	err = en.WriteMapHeader(uint32(len(z.Map)))
	if err != nil {
		err = msgp.WrapError(err, "Map")
		return
	}
	for za0001, za0002 := range z.Map {
		err = en.WriteString(za0001)
		if err != nil {
			err = msgp.WrapError(err, "Map")
			return
		}
		err = en.WriteInt(za0002)
		if err != nil {
			err = msgp.WrapError(err, "Map", za0001)
			return
		}
	}
	// write "Array"
	err = en.Append(0xa5, 0x41, 0x72, 0x72, 0x61, 0x79)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.Array)))
	if err != nil {
		err = msgp.WrapError(err, "Array")
		return
	}
	for za0003 := range z.Array {
		err = en.WriteString(z.Array[za0003])
		if err != nil {
			err = msgp.WrapError(err, "Array", za0003)
			return
		}
	}
	// write "ArrayRange"
	err = en.Append(0xaa, 0x41, 0x72, 0x72, 0x61, 0x79, 0x52, 0x61, 0x6e, 0x67, 0x65)
	if err != nil {
		return
	}
	err = en.WriteArrayHeader(uint32(len(z.ArrayRange)))
	if err != nil {
		err = msgp.WrapError(err, "ArrayRange")
		return
	}
	for za0004 := range z.ArrayRange {
		err = en.WriteString(z.ArrayRange[za0004])
		if err != nil {
			err = msgp.WrapError(err, "ArrayRange", za0004)
			return
		}
	}
	// write "Bar"
	err = en.Append(0xa3, 0x42, 0x61, 0x72)
	if err != nil {
		return
	}
	// map header, size 3
	// write "Name"
	err = en.Append(0x83, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return
	}
	err = en.WriteString(z.Bar.Name)
	if err != nil {
		err = msgp.WrapError(err, "Bar", "Name")
		return
	}
	// write "Number"
	err = en.Append(0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	if err != nil {
		return
	}
	err = en.WriteInt(z.Bar.Number)
	if err != nil {
		err = msgp.WrapError(err, "Bar", "Number")
		return
	}
	// write "Float"
	err = en.Append(0xa5, 0x46, 0x6c, 0x6f, 0x61, 0x74)
	if err != nil {
		return
	}
	err = en.WriteFloat32(z.Bar.Float)
	if err != nil {
		err = msgp.WrapError(err, "Bar", "Float")
		return
	}
	// write "Skip"
	err = en.Append(0xa4, 0x53, 0x6b, 0x69, 0x70)
	if err != nil {
		return
	}
	if z.Skip == nil {
		err = en.WriteNil()
		if err != nil {
			return
		}
	} else {
		err = en.WriteString(*z.Skip)
		if err != nil {
			err = msgp.WrapError(err, "Skip")
			return
		}
	}
	// write "Created"
	err = en.Append(0xa7, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64)
	if err != nil {
		return
	}
	err = en.WriteTime(z.Created)
	if err != nil {
		err = msgp.WrapError(err, "Created")
		return
	}
	// write "CreatedFormat"
	err = en.Append(0xad, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x46, 0x6f, 0x72, 0x6d, 0x61, 0x74)
	if err != nil {
		return
	}
	err = en.WriteTime(z.CreatedFormat)
	if err != nil {
		err = msgp.WrapError(err, "CreatedFormat")
		return
	}
	// write "ByteData"
	err = en.Append(0xa8, 0x42, 0x79, 0x74, 0x65, 0x44, 0x61, 0x74, 0x61)
	if err != nil {
		return
	}
	err = en.WriteBytes(z.ByteData)
	if err != nil {
		err = msgp.WrapError(err, "ByteData")
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *Foo) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 17
	// string "ID"
	o = append(o, 0xde, 0x0, 0x11, 0xa2, 0x49, 0x44)
	o = msgp.AppendInt(o, z.ID)
	// string "Str"
	o = append(o, 0xa3, 0x53, 0x74, 0x72)
	o = msgp.AppendString(o, z.Str)
	// string "Int"
	o = append(o, 0xa3, 0x49, 0x6e, 0x74)
	o = msgp.AppendInt(o, z.Int)
	// string "Pointer"
	o = append(o, 0xa7, 0x50, 0x6f, 0x69, 0x6e, 0x74, 0x65, 0x72)
	if z.Pointer == nil {
		o = msgp.AppendNil(o)
	} else {
		o = msgp.AppendInt(o, *z.Pointer)
	}
	// string "Name"
	o = append(o, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Sentence"
	o = append(o, 0xa8, 0x53, 0x65, 0x6e, 0x74, 0x65, 0x6e, 0x63, 0x65)
	o = msgp.AppendString(o, z.Sentence)
	// string "RandStr"
	o = append(o, 0xa7, 0x52, 0x61, 0x6e, 0x64, 0x53, 0x74, 0x72)
	o = msgp.AppendString(o, z.RandStr)
	// string "Number"
	o = append(o, 0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	o = msgp.AppendString(o, z.Number)
	// string "Regex"
	o = append(o, 0xa5, 0x52, 0x65, 0x67, 0x65, 0x78)
	o = msgp.AppendString(o, z.Regex)
	// string "Map"
	o = append(o, 0xa3, 0x4d, 0x61, 0x70)
	o = msgp.AppendMapHeader(o, uint32(len(z.Map)))
	for za0001, za0002 := range z.Map {
		o = msgp.AppendString(o, za0001)
		o = msgp.AppendInt(o, za0002)
	}
	// string "Array"
	o = append(o, 0xa5, 0x41, 0x72, 0x72, 0x61, 0x79)
	o = msgp.AppendArrayHeader(o, uint32(len(z.Array)))
	for za0003 := range z.Array {
		o = msgp.AppendString(o, z.Array[za0003])
	}
	// string "ArrayRange"
	o = append(o, 0xaa, 0x41, 0x72, 0x72, 0x61, 0x79, 0x52, 0x61, 0x6e, 0x67, 0x65)
	o = msgp.AppendArrayHeader(o, uint32(len(z.ArrayRange)))
	for za0004 := range z.ArrayRange {
		o = msgp.AppendString(o, z.ArrayRange[za0004])
	}
	// string "Bar"
	o = append(o, 0xa3, 0x42, 0x61, 0x72)
	// map header, size 3
	// string "Name"
	o = append(o, 0x83, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Bar.Name)
	// string "Number"
	o = append(o, 0xa6, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72)
	o = msgp.AppendInt(o, z.Bar.Number)
	// string "Float"
	o = append(o, 0xa5, 0x46, 0x6c, 0x6f, 0x61, 0x74)
	o = msgp.AppendFloat32(o, z.Bar.Float)
	// string "Skip"
	o = append(o, 0xa4, 0x53, 0x6b, 0x69, 0x70)
	if z.Skip == nil {
		o = msgp.AppendNil(o)
	} else {
		o = msgp.AppendString(o, *z.Skip)
	}
	// string "Created"
	o = append(o, 0xa7, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64)
	o = msgp.AppendTime(o, z.Created)
	// string "CreatedFormat"
	o = append(o, 0xad, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x46, 0x6f, 0x72, 0x6d, 0x61, 0x74)
	o = msgp.AppendTime(o, z.CreatedFormat)
	// string "ByteData"
	o = append(o, 0xa8, 0x42, 0x79, 0x74, 0x65, 0x44, 0x61, 0x74, 0x61)
	o = msgp.AppendBytes(o, z.ByteData)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *Foo) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			z.ID, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ID")
				return
			}
		case "Str":
			z.Str, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Str")
				return
			}
		case "Int":
			z.Int, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Int")
				return
			}
		case "Pointer":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Pointer = nil
			} else {
				if z.Pointer == nil {
					z.Pointer = new(int)
				}
				*z.Pointer, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Pointer")
					return
				}
			}
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Name")
				return
			}
		case "Sentence":
			z.Sentence, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Sentence")
				return
			}
		case "RandStr":
			z.RandStr, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "RandStr")
				return
			}
		case "Number":
			z.Number, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Number")
				return
			}
		case "Regex":
			z.Regex, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Regex")
				return
			}
		case "Map":
			var zb0002 uint32
			zb0002, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Map")
				return
			}
			if z.Map == nil {
				z.Map = make(map[string]int, zb0002)
			} else if len(z.Map) > 0 {
				for key := range z.Map {
					delete(z.Map, key)
				}
			}
			for zb0002 > 0 {
				var za0001 string
				var za0002 int
				zb0002--
				za0001, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Map")
					return
				}
				za0002, bts, err = msgp.ReadIntBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Map", za0001)
					return
				}
				z.Map[za0001] = za0002
			}
		case "Array":
			var zb0003 uint32
			zb0003, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Array")
				return
			}
			if cap(z.Array) >= int(zb0003) {
				z.Array = (z.Array)[:zb0003]
			} else {
				z.Array = make([]string, zb0003)
			}
			for za0003 := range z.Array {
				z.Array[za0003], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Array", za0003)
					return
				}
			}
		case "ArrayRange":
			var zb0004 uint32
			zb0004, bts, err = msgp.ReadArrayHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "ArrayRange")
				return
			}
			if cap(z.ArrayRange) >= int(zb0004) {
				z.ArrayRange = (z.ArrayRange)[:zb0004]
			} else {
				z.ArrayRange = make([]string, zb0004)
			}
			for za0004 := range z.ArrayRange {
				z.ArrayRange[za0004], bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "ArrayRange", za0004)
					return
				}
			}
		case "Bar":
			var zb0005 uint32
			zb0005, bts, err = msgp.ReadMapHeaderBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Bar")
				return
			}
			for zb0005 > 0 {
				zb0005--
				field, bts, err = msgp.ReadMapKeyZC(bts)
				if err != nil {
					err = msgp.WrapError(err, "Bar")
					return
				}
				switch msgp.UnsafeString(field) {
				case "Name":
					z.Bar.Name, bts, err = msgp.ReadStringBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Name")
						return
					}
				case "Number":
					z.Bar.Number, bts, err = msgp.ReadIntBytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Number")
						return
					}
				case "Float":
					z.Bar.Float, bts, err = msgp.ReadFloat32Bytes(bts)
					if err != nil {
						err = msgp.WrapError(err, "Bar", "Float")
						return
					}
				default:
					bts, err = msgp.Skip(bts)
					if err != nil {
						err = msgp.WrapError(err, "Bar")
						return
					}
				}
			}
		case "Skip":
			if msgp.IsNil(bts) {
				bts, err = msgp.ReadNilBytes(bts)
				if err != nil {
					return
				}
				z.Skip = nil
			} else {
				if z.Skip == nil {
					z.Skip = new(string)
				}
				*z.Skip, bts, err = msgp.ReadStringBytes(bts)
				if err != nil {
					err = msgp.WrapError(err, "Skip")
					return
				}
			}
		case "Created":
			z.Created, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "Created")
				return
			}
		case "CreatedFormat":
			z.CreatedFormat, bts, err = msgp.ReadTimeBytes(bts)
			if err != nil {
				err = msgp.WrapError(err, "CreatedFormat")
				return
			}
		case "ByteData":
			z.ByteData, bts, err = msgp.ReadBytesBytes(bts, z.ByteData)
			if err != nil {
				err = msgp.WrapError(err, "ByteData")
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *Foo) Msgsize() (s int) {
	s = 3 + 3 + msgp.IntSize + 4 + msgp.StringPrefixSize + len(z.Str) + 4 + msgp.IntSize + 8
	if z.Pointer == nil {
		s += msgp.NilSize
	} else {
		s += msgp.IntSize
	}
	s += 5 + msgp.StringPrefixSize + len(z.Name) + 9 + msgp.StringPrefixSize + len(z.Sentence) + 8 + msgp.StringPrefixSize + len(z.RandStr) + 7 + msgp.StringPrefixSize + len(z.Number) + 6 + msgp.StringPrefixSize + len(z.Regex) + 4 + msgp.MapHeaderSize
	if z.Map != nil {
		for za0001, za0002 := range z.Map {
			_ = za0002
			s += msgp.StringPrefixSize + len(za0001) + msgp.IntSize
		}
	}
	s += 6 + msgp.ArrayHeaderSize
	for za0003 := range z.Array {
		s += msgp.StringPrefixSize + len(z.Array[za0003])
	}
	s += 11 + msgp.ArrayHeaderSize
	for za0004 := range z.ArrayRange {
		s += msgp.StringPrefixSize + len(z.ArrayRange[za0004])
	}
	s += 4 + 1 + 5 + msgp.StringPrefixSize + len(z.Bar.Name) + 7 + msgp.IntSize + 6 + msgp.Float32Size + 5
	if z.Skip == nil {
		s += msgp.NilSize
	} else {
		s += msgp.StringPrefixSize + len(*z.Skip)
	}
	s += 8 + msgp.TimeSize + 14 + msgp.TimeSize + 9 + msgp.BytesPrefixSize + len(z.ByteData)
	return
}

// DecodeMsg implements msgp.Decodable
func (z *MsgpSerializer) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, err = dc.ReadMapHeader()
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		default:
			err = dc.Skip()
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z MsgpSerializer) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 0
	err = en.Append(0x80)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z MsgpSerializer) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 0
	o = append(o, 0x80)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *MsgpSerializer) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zb0001 uint32
	zb0001, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		err = msgp.WrapError(err)
		return
	}
	for zb0001 > 0 {
		zb0001--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			err = msgp.WrapError(err)
			return
		}
		switch msgp.UnsafeString(field) {
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				err = msgp.WrapError(err)
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z MsgpSerializer) Msgsize() (s int) {
	s = 1
	return
}
