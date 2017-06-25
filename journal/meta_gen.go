package journal

// NOTE: THIS FILE WAS PRODUCED BY THE
// MSGP CODE GENERATION TOOL (github.com/tinylib/msgp)
// DO NOT EDIT

import "github.com/tinylib/msgp/msgp"

// DecodeMsg implements msgp.Decodable
func (z *ConsistencyLevel) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zxvk int
		zxvk, err = dc.ReadInt()
		(*z) = ConsistencyLevel(zxvk)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z ConsistencyLevel) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteInt(int(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ConsistencyLevel) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendInt(o, int(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ConsistencyLevel) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zbzg int
		zbzg, bts, err = msgp.ReadIntBytes(bts)
		(*z) = ConsistencyLevel(zbzg)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z ConsistencyLevel) Msgsize() (s int) {
	s = msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *FileMeta) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zbai uint32
	zbai, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zbai > 0 {
		zbai--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			z.ID, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Name":
			z.Name, err = dc.ReadString()
			if err != nil {
				return
			}
		case "Size":
			z.Size, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "Timestamp":
			z.Timestamp, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "UserMeta":
			z.UserMeta, err = dc.ReadIntf()
			if err != nil {
				return
			}
		case "IsSymlink":
			z.IsSymlink, err = dc.ReadBool()
			if err != nil {
				return
			}
		case "Consistency":
			{
				var zcmr int
				zcmr, err = dc.ReadInt()
				z.Consistency = ConsistencyLevel(zcmr)
			}
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *FileMeta) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 7
	// write "ID"
	err = en.Append(0x87, 0xa2, 0x49, 0x44)
	if err != nil {
		return err
	}
	err = en.WriteString(z.ID)
	if err != nil {
		return
	}
	// write "Name"
	err = en.Append(0xa4, 0x4e, 0x61, 0x6d, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteString(z.Name)
	if err != nil {
		return
	}
	// write "Size"
	err = en.Append(0xa4, 0x53, 0x69, 0x7a, 0x65)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.Size)
	if err != nil {
		return
	}
	// write "Timestamp"
	err = en.Append(0xa9, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.Timestamp)
	if err != nil {
		return
	}
	// write "UserMeta"
	err = en.Append(0xa8, 0x55, 0x73, 0x65, 0x72, 0x4d, 0x65, 0x74, 0x61)
	if err != nil {
		return err
	}
	err = en.WriteIntf(z.UserMeta)
	if err != nil {
		return
	}
	// write "IsSymlink"
	err = en.Append(0xa9, 0x49, 0x73, 0x53, 0x79, 0x6d, 0x6c, 0x69, 0x6e, 0x6b)
	if err != nil {
		return err
	}
	err = en.WriteBool(z.IsSymlink)
	if err != nil {
		return
	}
	// write "Consistency"
	err = en.Append(0xab, 0x43, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x63, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteInt(int(z.Consistency))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *FileMeta) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 7
	// string "ID"
	o = append(o, 0x87, 0xa2, 0x49, 0x44)
	o = msgp.AppendString(o, z.ID)
	// string "Name"
	o = append(o, 0xa4, 0x4e, 0x61, 0x6d, 0x65)
	o = msgp.AppendString(o, z.Name)
	// string "Size"
	o = append(o, 0xa4, 0x53, 0x69, 0x7a, 0x65)
	o = msgp.AppendInt64(o, z.Size)
	// string "Timestamp"
	o = append(o, 0xa9, 0x54, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70)
	o = msgp.AppendInt64(o, z.Timestamp)
	// string "UserMeta"
	o = append(o, 0xa8, 0x55, 0x73, 0x65, 0x72, 0x4d, 0x65, 0x74, 0x61)
	o, err = msgp.AppendIntf(o, z.UserMeta)
	if err != nil {
		return
	}
	// string "IsSymlink"
	o = append(o, 0xa9, 0x49, 0x73, 0x53, 0x79, 0x6d, 0x6c, 0x69, 0x6e, 0x6b)
	o = msgp.AppendBool(o, z.IsSymlink)
	// string "Consistency"
	o = append(o, 0xab, 0x43, 0x6f, 0x6e, 0x73, 0x69, 0x73, 0x74, 0x65, 0x6e, 0x63, 0x79)
	o = msgp.AppendInt(o, int(z.Consistency))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *FileMeta) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zajw uint32
	zajw, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zajw > 0 {
		zajw--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			z.ID, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Name":
			z.Name, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "Size":
			z.Size, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "Timestamp":
			z.Timestamp, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "UserMeta":
			z.UserMeta, bts, err = msgp.ReadIntfBytes(bts)
			if err != nil {
				return
			}
		case "IsSymlink":
			z.IsSymlink, bts, err = msgp.ReadBoolBytes(bts)
			if err != nil {
				return
			}
		case "Consistency":
			{
				var zwht int
				zwht, bts, err = msgp.ReadIntBytes(bts)
				z.Consistency = ConsistencyLevel(zwht)
			}
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *FileMeta) Msgsize() (s int) {
	s = 1 + 3 + msgp.StringPrefixSize + len(z.ID) + 5 + msgp.StringPrefixSize + len(z.Name) + 5 + msgp.Int64Size + 10 + msgp.Int64Size + 9 + msgp.GuessSize(z.UserMeta) + 10 + msgp.BoolSize + 12 + msgp.IntSize
	return
}

// DecodeMsg implements msgp.Decodable
func (z *ID) DecodeMsg(dc *msgp.Reader) (err error) {
	{
		var zhct string
		zhct, err = dc.ReadString()
		(*z) = ID(zhct)
	}
	if err != nil {
		return
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z ID) EncodeMsg(en *msgp.Writer) (err error) {
	err = en.WriteString(string(z))
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z ID) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	o = msgp.AppendString(o, string(z))
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *ID) UnmarshalMsg(bts []byte) (o []byte, err error) {
	{
		var zcua string
		zcua, bts, err = msgp.ReadStringBytes(bts)
		(*z) = ID(zcua)
	}
	if err != nil {
		return
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z ID) Msgsize() (s int) {
	s = msgp.StringPrefixSize + len(string(z))
	return
}

// DecodeMsg implements msgp.Decodable
func (z *JournalMeta) DecodeMsg(dc *msgp.Reader) (err error) {
	var field []byte
	_ = field
	var zxhx uint32
	zxhx, err = dc.ReadMapHeader()
	if err != nil {
		return
	}
	for zxhx > 0 {
		zxhx--
		field, err = dc.ReadMapKeyPtr()
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			{
				var zlqf string
				zlqf, err = dc.ReadString()
				z.ID = ID(zlqf)
			}
			if err != nil {
				return
			}
		case "CreatedAt":
			z.CreatedAt, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "JoinedAt":
			z.JoinedAt, err = dc.ReadInt64()
			if err != nil {
				return
			}
		case "FirstKey":
			z.FirstKey, err = dc.ReadString()
			if err != nil {
				return
			}
		case "LastKey":
			z.LastKey, err = dc.ReadString()
			if err != nil {
				return
			}
		case "CountTotal":
			z.CountTotal, err = dc.ReadInt()
			if err != nil {
				return
			}
		default:
			err = dc.Skip()
			if err != nil {
				return
			}
		}
	}
	return
}

// EncodeMsg implements msgp.Encodable
func (z *JournalMeta) EncodeMsg(en *msgp.Writer) (err error) {
	// map header, size 6
	// write "ID"
	err = en.Append(0x86, 0xa2, 0x49, 0x44)
	if err != nil {
		return err
	}
	err = en.WriteString(string(z.ID))
	if err != nil {
		return
	}
	// write "CreatedAt"
	err = en.Append(0xa9, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.CreatedAt)
	if err != nil {
		return
	}
	// write "JoinedAt"
	err = en.Append(0xa8, 0x4a, 0x6f, 0x69, 0x6e, 0x65, 0x64, 0x41, 0x74)
	if err != nil {
		return err
	}
	err = en.WriteInt64(z.JoinedAt)
	if err != nil {
		return
	}
	// write "FirstKey"
	err = en.Append(0xa8, 0x46, 0x69, 0x72, 0x73, 0x74, 0x4b, 0x65, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteString(z.FirstKey)
	if err != nil {
		return
	}
	// write "LastKey"
	err = en.Append(0xa7, 0x4c, 0x61, 0x73, 0x74, 0x4b, 0x65, 0x79)
	if err != nil {
		return err
	}
	err = en.WriteString(z.LastKey)
	if err != nil {
		return
	}
	// write "CountTotal"
	err = en.Append(0xaa, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x54, 0x6f, 0x74, 0x61, 0x6c)
	if err != nil {
		return err
	}
	err = en.WriteInt(z.CountTotal)
	if err != nil {
		return
	}
	return
}

// MarshalMsg implements msgp.Marshaler
func (z *JournalMeta) MarshalMsg(b []byte) (o []byte, err error) {
	o = msgp.Require(b, z.Msgsize())
	// map header, size 6
	// string "ID"
	o = append(o, 0x86, 0xa2, 0x49, 0x44)
	o = msgp.AppendString(o, string(z.ID))
	// string "CreatedAt"
	o = append(o, 0xa9, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74)
	o = msgp.AppendInt64(o, z.CreatedAt)
	// string "JoinedAt"
	o = append(o, 0xa8, 0x4a, 0x6f, 0x69, 0x6e, 0x65, 0x64, 0x41, 0x74)
	o = msgp.AppendInt64(o, z.JoinedAt)
	// string "FirstKey"
	o = append(o, 0xa8, 0x46, 0x69, 0x72, 0x73, 0x74, 0x4b, 0x65, 0x79)
	o = msgp.AppendString(o, z.FirstKey)
	// string "LastKey"
	o = append(o, 0xa7, 0x4c, 0x61, 0x73, 0x74, 0x4b, 0x65, 0x79)
	o = msgp.AppendString(o, z.LastKey)
	// string "CountTotal"
	o = append(o, 0xaa, 0x43, 0x6f, 0x75, 0x6e, 0x74, 0x54, 0x6f, 0x74, 0x61, 0x6c)
	o = msgp.AppendInt(o, z.CountTotal)
	return
}

// UnmarshalMsg implements msgp.Unmarshaler
func (z *JournalMeta) UnmarshalMsg(bts []byte) (o []byte, err error) {
	var field []byte
	_ = field
	var zdaf uint32
	zdaf, bts, err = msgp.ReadMapHeaderBytes(bts)
	if err != nil {
		return
	}
	for zdaf > 0 {
		zdaf--
		field, bts, err = msgp.ReadMapKeyZC(bts)
		if err != nil {
			return
		}
		switch msgp.UnsafeString(field) {
		case "ID":
			{
				var zpks string
				zpks, bts, err = msgp.ReadStringBytes(bts)
				z.ID = ID(zpks)
			}
			if err != nil {
				return
			}
		case "CreatedAt":
			z.CreatedAt, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "JoinedAt":
			z.JoinedAt, bts, err = msgp.ReadInt64Bytes(bts)
			if err != nil {
				return
			}
		case "FirstKey":
			z.FirstKey, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "LastKey":
			z.LastKey, bts, err = msgp.ReadStringBytes(bts)
			if err != nil {
				return
			}
		case "CountTotal":
			z.CountTotal, bts, err = msgp.ReadIntBytes(bts)
			if err != nil {
				return
			}
		default:
			bts, err = msgp.Skip(bts)
			if err != nil {
				return
			}
		}
	}
	o = bts
	return
}

// Msgsize returns an upper bound estimate of the number of bytes occupied by the serialized message
func (z *JournalMeta) Msgsize() (s int) {
	s = 1 + 3 + msgp.StringPrefixSize + len(string(z.ID)) + 10 + msgp.Int64Size + 9 + msgp.Int64Size + 9 + msgp.StringPrefixSize + len(z.FirstKey) + 8 + msgp.StringPrefixSize + len(z.LastKey) + 11 + msgp.IntSize
	return
}
