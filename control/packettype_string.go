// Code generated by "stringer -type PacketType"; DO NOT EDIT.

package control

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[PacketTypeSync-0]
	_ = x[PacketTypeResponse-1]
	_ = x[PacketTypeClearCache-2]
	_ = x[PacketTypeListDrivers-3]
}

const _PacketType_name = "PacketTypeSyncPacketTypeResponsePacketTypeClearCachePacketTypeListDrivers"

var _PacketType_index = [...]uint8{0, 14, 32, 52, 73}

func (i PacketType) String() string {
	if i < 0 || i >= PacketType(len(_PacketType_index)-1) {
		return "PacketType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _PacketType_name[_PacketType_index[i]:_PacketType_index[i+1]]
}
