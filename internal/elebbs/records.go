package elebbs

const (
	FilesHdrSize = 194
	FilesIdxSize = 41
	FilesRecSize = 168
	TagRecSize   = 50 // EleBBS TagFileRecord; LFN via AreaNum+RecordNum
	GroupRecSize = 153
	EleFilesSize = 0 // unused; EleFilesRecord is variable-ish with Strings

	AttrDeleted  = 1 << 0
	AttrUnlisted = 1 << 1
	AttrFree     = 1 << 2
	AttrNotAvail = 1 << 3
	AttrLocked   = 1 << 4
	AttrMissing  = 1 << 5
	AttrNoTime   = 1 << 6

	AreaAttrNewScan  = 1 << 0
	AreaAttrDupeScan = 1 << 1
	AreaAttrLongDesc = 1 << 2
	AreaAttrCDROM    = 1 << 3
	AreaAttrAllFree  = 1 << 4
	AreaAttrNotInFDB = 1 << 5
	AreaAttrPwdUL    = 1 << 6
	AreaAttrScanUL   = 1 << 7

	MaxTagged = 100
)

type FilesHdr struct {
	Name        string
	NameRaw     [13]byte
	Size        uint32
	CRC32       uint32
	Uploader    string
	UploadDate  uint32
	FileDate    uint32
	LastDL      uint32
	TimesDL     uint16
	Attrib      byte
	Password    string
	Keywords    [5]string
	Cost        uint16
	LongDescPtr int32
	LfnPtr      int32
	RecordNum   uint16
}

func (h FilesHdr) Deleted() bool  { return h.Attrib&AttrDeleted != 0 }
func (h FilesHdr) Unlisted() bool { return h.Attrib&AttrUnlisted != 0 }
func (h FilesHdr) Missing() bool  { return h.Attrib&AttrMissing != 0 }
func (h FilesHdr) NotAvail() bool { return h.Attrib&AttrNotAvail != 0 }
func (h FilesHdr) Comment() bool  { return h.NameRaw[0] == 0 }

func parseFilesHdr(b []byte, rec uint16) FilesHdr {
	var h FilesHdr
	if len(b) < FilesHdrSize {
		return h
	}
	copy(h.NameRaw[:], b[0:13])
	h.Name = pascalString(b[0:13])
	h.Size = u32(b, 13)
	h.CRC32 = u32(b, 17)
	h.Uploader = pascalString(b[21:57])
	h.UploadDate = u32(b, 57)
	h.FileDate = u32(b, 61)
	h.LastDL = u32(b, 65)
	h.TimesDL = u16(b, 69)
	h.Attrib = b[71]
	h.Password = pascalString(b[72:88])
	off := 88
	for i := 0; i < 5; i++ {
		h.Keywords[i] = pascalString(b[off : off+16])
		off += 16
	}
	h.Cost = u16(b, 168)
	h.LongDescPtr = i32(b, 170)
	h.LfnPtr = i32(b, 174)
	h.RecordNum = rec
	return h
}

func encodeFilesHdr(h FilesHdr) []byte {
	b := make([]byte, FilesHdrSize)
	copy(b[0:13], h.NameRaw[:])
	putU32(b, 13, h.Size)
	putU32(b, 17, h.CRC32)
	putPascal(b[21:57], h.Uploader)
	putU32(b, 57, h.UploadDate)
	putU32(b, 61, h.FileDate)
	putU32(b, 65, h.LastDL)
	putU16(b, 69, h.TimesDL)
	b[71] = h.Attrib
	putPascal(b[72:88], h.Password)
	off := 88
	for i := 0; i < 5; i++ {
		putPascal(b[off:off+16], h.Keywords[i])
		off += 16
	}
	putU16(b, 168, h.Cost)
	putU32(b, 170, uint32(h.LongDescPtr))
	putU32(b, 174, uint32(h.LfnPtr))
	return b
}

type FilesArea struct {
	AreaNum        uint16
	Name           string
	Attrib         byte
	FilePath       string
	KillDaysDL     uint16
	KillDaysFD     uint16
	Password       string
	MoveArea       uint16
	Age            byte
	ConvertExt     byte
	Group          uint16
	Attrib2        byte
	DefCost        uint16
	UploadArea     uint16
	UploadSecurity uint16
	Security       uint16
	ListSecurity   uint16
	AltGroup       [3]uint16
	Device         byte
}

func parseFilesArea(b []byte) FilesArea {
	var a FilesArea
	if len(b) < FilesRecSize {
		return a
	}
	a.AreaNum = u16(b, 0)
	a.Name = pascalString(b[4:45])
	a.Attrib = b[45]
	a.FilePath = pascalString(b[46:87])
	a.KillDaysDL = u16(b, 87)
	a.KillDaysFD = u16(b, 89)
	a.Password = pascalString(b[91:107])
	a.MoveArea = u16(b, 107)
	a.Age = b[109]
	a.ConvertExt = b[110]
	a.Group = u16(b, 111)
	a.Attrib2 = b[113]
	a.DefCost = u16(b, 114)
	a.UploadArea = u16(b, 116)
	a.UploadSecurity = u16(b, 118)
	a.Security = u16(b, 128)
	a.ListSecurity = u16(b, 140)
	a.AltGroup[0] = u16(b, 148)
	a.AltGroup[1] = u16(b, 150)
	a.AltGroup[2] = u16(b, 152)
	a.Device = b[154]
	return a
}

type Group struct {
	AreaNum  uint16
	Name     string
	Security uint16
}

func parseGroup(b []byte) Group {
	var g Group
	if len(b) < GroupRecSize {
		return g
	}
	g.AreaNum = u16(b, 0)
	g.Name = pascalString(b[2:43])
	g.Security = u16(b, 43)
	return g
}

type TagRecord struct {
	Name       string
	NameRaw    [13]byte
	Lfn        string
	Password   string
	Attrib     byte
	AreaNum    uint16
	RecordNum  uint16
	Size       uint32
	FileDate   uint32
	Cost       uint32
	CDROM      bool
	FoundFirst bool
	XferTime   uint16
}

func parseTag(b []byte) TagRecord {
	var t TagRecord
	if len(b) < TagRecSize {
		return t
	}
	copy(t.NameRaw[:], b[0:13])
	t.Name = pascalString(b[0:13])
	t.Password = pascalString(b[13:29])
	t.Attrib = b[29]
	t.AreaNum = u16(b, 30)
	t.RecordNum = u16(b, 32)
	t.Size = u32(b, 34)
	t.FileDate = u32(b, 38)
	t.Cost = u32(b, 42)
	t.CDROM = b[46] != 0
	t.FoundFirst = b[47] != 0
	t.XferTime = u16(b, 48)
	return t
}

func encodeTag(t TagRecord) []byte {
	b := make([]byte, TagRecSize)
	if t.NameRaw[0] != 0 {
		copy(b[0:13], t.NameRaw[:])
	} else {
		putPascal(b[0:13], t.Name)
	}
	putPascal(b[13:29], t.Password)
	b[29] = t.Attrib
	putU16(b, 30, t.AreaNum)
	putU16(b, 32, t.RecordNum)
	putU32(b, 34, t.Size)
	putU32(b, 38, t.FileDate)
	putU32(b, 42, t.Cost)
	if t.CDROM {
		b[46] = 1
	}
	if t.FoundFirst {
		b[47] = 1
	}
	putU16(b, 48, t.XferTime)
	return b
}

func TagFromHdr(h FilesHdr, areaNum uint16, areaAttr byte, fileName string) TagRecord {
	t := TagRecord{
		Name:      h.Name,
		NameRaw:   h.NameRaw,
		Lfn:       fileName,
		Password:  h.Password,
		Attrib:    h.Attrib,
		AreaNum:   areaNum,
		RecordNum: h.RecordNum,
		Size:      h.Size,
		FileDate:  h.FileDate,
		Cost:      uint32(h.Cost),
		CDROM:     areaAttr&AreaAttrCDROM != 0,
	}
	if t.NameRaw[0] == 0 && h.Name != "" {
		putPascal(t.NameRaw[:], h.Name)
	}
	if t.Name == "" {
		t.Name = h.Name
	}
	return t
}
