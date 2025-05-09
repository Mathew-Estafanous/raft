// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v3.17.3
// source: raft.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ApplyResult int32

const (
	ApplyResult_Committed ApplyResult = 0
	ApplyResult_Failed    ApplyResult = 1
)

// Enum value maps for ApplyResult.
var (
	ApplyResult_name = map[int32]string{
		0: "Committed",
		1: "Failed",
	}
	ApplyResult_value = map[string]int32{
		"Committed": 0,
		"Failed":    1,
	}
)

func (x ApplyResult) Enum() *ApplyResult {
	p := new(ApplyResult)
	*p = x
	return p
}

func (x ApplyResult) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ApplyResult) Descriptor() protoreflect.EnumDescriptor {
	return file_raft_proto_enumTypes[0].Descriptor()
}

func (ApplyResult) Type() protoreflect.EnumType {
	return &file_raft_proto_enumTypes[0]
}

func (x ApplyResult) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ApplyResult.Descriptor instead.
func (ApplyResult) EnumDescriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{0}
}

type VoteRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Term          uint64                 `protobuf:"varint,1,opt,name=term,proto3" json:"term,omitempty"`
	CandidateId   uint64                 `protobuf:"varint,2,opt,name=candidateId,proto3" json:"candidateId,omitempty"`
	LastLogIndex  int64                  `protobuf:"varint,3,opt,name=lastLogIndex,proto3" json:"lastLogIndex,omitempty"`
	LastLogTerm   uint64                 `protobuf:"varint,4,opt,name=lastLogTerm,proto3" json:"lastLogTerm,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *VoteRequest) Reset() {
	*x = VoteRequest{}
	mi := &file_raft_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *VoteRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VoteRequest) ProtoMessage() {}

func (x *VoteRequest) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VoteRequest.ProtoReflect.Descriptor instead.
func (*VoteRequest) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{0}
}

func (x *VoteRequest) GetTerm() uint64 {
	if x != nil {
		return x.Term
	}
	return 0
}

func (x *VoteRequest) GetCandidateId() uint64 {
	if x != nil {
		return x.CandidateId
	}
	return 0
}

func (x *VoteRequest) GetLastLogIndex() int64 {
	if x != nil {
		return x.LastLogIndex
	}
	return 0
}

func (x *VoteRequest) GetLastLogTerm() uint64 {
	if x != nil {
		return x.LastLogTerm
	}
	return 0
}

type VoteResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Term          uint64                 `protobuf:"varint,1,opt,name=term,proto3" json:"term,omitempty"`
	VoteGranted   bool                   `protobuf:"varint,2,opt,name=voteGranted,proto3" json:"voteGranted,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *VoteResponse) Reset() {
	*x = VoteResponse{}
	mi := &file_raft_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *VoteResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VoteResponse) ProtoMessage() {}

func (x *VoteResponse) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VoteResponse.ProtoReflect.Descriptor instead.
func (*VoteResponse) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{1}
}

func (x *VoteResponse) GetTerm() uint64 {
	if x != nil {
		return x.Term
	}
	return 0
}

func (x *VoteResponse) GetVoteGranted() bool {
	if x != nil {
		return x.VoteGranted
	}
	return false
}

type Entry struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Term          uint64                 `protobuf:"varint,1,opt,name=term,proto3" json:"term,omitempty"`
	Index         int64                  `protobuf:"varint,2,opt,name=index,proto3" json:"index,omitempty"`
	Data          []byte                 `protobuf:"bytes,3,opt,name=data,proto3" json:"data,omitempty"`
	Type          []byte                 `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Entry) Reset() {
	*x = Entry{}
	mi := &file_raft_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Entry) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Entry) ProtoMessage() {}

func (x *Entry) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Entry.ProtoReflect.Descriptor instead.
func (*Entry) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{2}
}

func (x *Entry) GetTerm() uint64 {
	if x != nil {
		return x.Term
	}
	return 0
}

func (x *Entry) GetIndex() int64 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *Entry) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *Entry) GetType() []byte {
	if x != nil {
		return x.Type
	}
	return nil
}

type AppendEntriesRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Term          uint64                 `protobuf:"varint,1,opt,name=term,proto3" json:"term,omitempty"`
	LeaderId      uint64                 `protobuf:"varint,2,opt,name=leaderId,proto3" json:"leaderId,omitempty"`
	PrevLogIndex  int64                  `protobuf:"varint,3,opt,name=prevLogIndex,proto3" json:"prevLogIndex,omitempty"`
	PrevLogTerm   uint64                 `protobuf:"varint,4,opt,name=prevLogTerm,proto3" json:"prevLogTerm,omitempty"`
	LeaderCommit  int64                  `protobuf:"varint,6,opt,name=leaderCommit,proto3" json:"leaderCommit,omitempty"`
	Entries       []*Entry               `protobuf:"bytes,5,rep,name=entries,proto3" json:"entries,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AppendEntriesRequest) Reset() {
	*x = AppendEntriesRequest{}
	mi := &file_raft_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendEntriesRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendEntriesRequest) ProtoMessage() {}

func (x *AppendEntriesRequest) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendEntriesRequest.ProtoReflect.Descriptor instead.
func (*AppendEntriesRequest) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{3}
}

func (x *AppendEntriesRequest) GetTerm() uint64 {
	if x != nil {
		return x.Term
	}
	return 0
}

func (x *AppendEntriesRequest) GetLeaderId() uint64 {
	if x != nil {
		return x.LeaderId
	}
	return 0
}

func (x *AppendEntriesRequest) GetPrevLogIndex() int64 {
	if x != nil {
		return x.PrevLogIndex
	}
	return 0
}

func (x *AppendEntriesRequest) GetPrevLogTerm() uint64 {
	if x != nil {
		return x.PrevLogTerm
	}
	return 0
}

func (x *AppendEntriesRequest) GetLeaderCommit() int64 {
	if x != nil {
		return x.LeaderCommit
	}
	return 0
}

func (x *AppendEntriesRequest) GetEntries() []*Entry {
	if x != nil {
		return x.Entries
	}
	return nil
}

type AppendEntriesResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            uint64                 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	Term          uint64                 `protobuf:"varint,2,opt,name=term,proto3" json:"term,omitempty"`
	Success       bool                   `protobuf:"varint,3,opt,name=success,proto3" json:"success,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AppendEntriesResponse) Reset() {
	*x = AppendEntriesResponse{}
	mi := &file_raft_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AppendEntriesResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppendEntriesResponse) ProtoMessage() {}

func (x *AppendEntriesResponse) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppendEntriesResponse.ProtoReflect.Descriptor instead.
func (*AppendEntriesResponse) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{4}
}

func (x *AppendEntriesResponse) GetId() uint64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *AppendEntriesResponse) GetTerm() uint64 {
	if x != nil {
		return x.Term
	}
	return 0
}

func (x *AppendEntriesResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

type ApplyRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Command       []byte                 `protobuf:"bytes,1,opt,name=command,proto3" json:"command,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ApplyRequest) Reset() {
	*x = ApplyRequest{}
	mi := &file_raft_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ApplyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyRequest) ProtoMessage() {}

func (x *ApplyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyRequest.ProtoReflect.Descriptor instead.
func (*ApplyRequest) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{5}
}

func (x *ApplyRequest) GetCommand() []byte {
	if x != nil {
		return x.Command
	}
	return nil
}

type ApplyResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Result        ApplyResult            `protobuf:"varint,1,opt,name=result,proto3,enum=pb.ApplyResult" json:"result,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *ApplyResponse) Reset() {
	*x = ApplyResponse{}
	mi := &file_raft_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ApplyResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ApplyResponse) ProtoMessage() {}

func (x *ApplyResponse) ProtoReflect() protoreflect.Message {
	mi := &file_raft_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ApplyResponse.ProtoReflect.Descriptor instead.
func (*ApplyResponse) Descriptor() ([]byte, []int) {
	return file_raft_proto_rawDescGZIP(), []int{6}
}

func (x *ApplyResponse) GetResult() ApplyResult {
	if x != nil {
		return x.Result
	}
	return ApplyResult_Committed
}

var File_raft_proto protoreflect.FileDescriptor

const file_raft_proto_rawDesc = "" +
	"\n" +
	"\n" +
	"raft.proto\x12\x02pb\"\x89\x01\n" +
	"\vVoteRequest\x12\x12\n" +
	"\x04term\x18\x01 \x01(\x04R\x04term\x12 \n" +
	"\vcandidateId\x18\x02 \x01(\x04R\vcandidateId\x12\"\n" +
	"\flastLogIndex\x18\x03 \x01(\x03R\flastLogIndex\x12 \n" +
	"\vlastLogTerm\x18\x04 \x01(\x04R\vlastLogTerm\"D\n" +
	"\fVoteResponse\x12\x12\n" +
	"\x04term\x18\x01 \x01(\x04R\x04term\x12 \n" +
	"\vvoteGranted\x18\x02 \x01(\bR\vvoteGranted\"Y\n" +
	"\x05Entry\x12\x12\n" +
	"\x04term\x18\x01 \x01(\x04R\x04term\x12\x14\n" +
	"\x05index\x18\x02 \x01(\x03R\x05index\x12\x12\n" +
	"\x04data\x18\x03 \x01(\fR\x04data\x12\x12\n" +
	"\x04type\x18\x04 \x01(\fR\x04type\"\xd5\x01\n" +
	"\x14AppendEntriesRequest\x12\x12\n" +
	"\x04term\x18\x01 \x01(\x04R\x04term\x12\x1a\n" +
	"\bleaderId\x18\x02 \x01(\x04R\bleaderId\x12\"\n" +
	"\fprevLogIndex\x18\x03 \x01(\x03R\fprevLogIndex\x12 \n" +
	"\vprevLogTerm\x18\x04 \x01(\x04R\vprevLogTerm\x12\"\n" +
	"\fleaderCommit\x18\x06 \x01(\x03R\fleaderCommit\x12#\n" +
	"\aentries\x18\x05 \x03(\v2\t.pb.EntryR\aentries\"U\n" +
	"\x15AppendEntriesResponse\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\x04R\x02id\x12\x12\n" +
	"\x04term\x18\x02 \x01(\x04R\x04term\x12\x18\n" +
	"\asuccess\x18\x03 \x01(\bR\asuccess\"(\n" +
	"\fApplyRequest\x12\x18\n" +
	"\acommand\x18\x01 \x01(\fR\acommand\"8\n" +
	"\rApplyResponse\x12'\n" +
	"\x06result\x18\x01 \x01(\x0e2\x0f.pb.ApplyResultR\x06result*(\n" +
	"\vApplyResult\x12\r\n" +
	"\tCommitted\x10\x00\x12\n" +
	"\n" +
	"\x06Failed\x10\x012\xb7\x01\n" +
	"\x04Raft\x122\n" +
	"\vRequestVote\x12\x0f.pb.VoteRequest\x1a\x10.pb.VoteResponse\"\x00\x12D\n" +
	"\vAppendEntry\x12\x18.pb.AppendEntriesRequest\x1a\x19.pb.AppendEntriesResponse\"\x00\x125\n" +
	"\fForwardApply\x12\x10.pb.ApplyRequest\x1a\x11.pb.ApplyResponse\"\x00B\aZ\x05../pbb\x06proto3"

var (
	file_raft_proto_rawDescOnce sync.Once
	file_raft_proto_rawDescData []byte
)

func file_raft_proto_rawDescGZIP() []byte {
	file_raft_proto_rawDescOnce.Do(func() {
		file_raft_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_raft_proto_rawDesc), len(file_raft_proto_rawDesc)))
	})
	return file_raft_proto_rawDescData
}

var file_raft_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_raft_proto_msgTypes = make([]protoimpl.MessageInfo, 7)
var file_raft_proto_goTypes = []any{
	(ApplyResult)(0),              // 0: pb.ApplyResult
	(*VoteRequest)(nil),           // 1: pb.VoteRequest
	(*VoteResponse)(nil),          // 2: pb.VoteResponse
	(*Entry)(nil),                 // 3: pb.Entry
	(*AppendEntriesRequest)(nil),  // 4: pb.AppendEntriesRequest
	(*AppendEntriesResponse)(nil), // 5: pb.AppendEntriesResponse
	(*ApplyRequest)(nil),          // 6: pb.ApplyRequest
	(*ApplyResponse)(nil),         // 7: pb.ApplyResponse
}
var file_raft_proto_depIdxs = []int32{
	3, // 0: pb.AppendEntriesRequest.entries:type_name -> pb.Entry
	0, // 1: pb.ApplyResponse.result:type_name -> pb.ApplyResult
	1, // 2: pb.Raft.RequestVote:input_type -> pb.VoteRequest
	4, // 3: pb.Raft.AppendEntry:input_type -> pb.AppendEntriesRequest
	6, // 4: pb.Raft.ForwardApply:input_type -> pb.ApplyRequest
	2, // 5: pb.Raft.RequestVote:output_type -> pb.VoteResponse
	5, // 6: pb.Raft.AppendEntry:output_type -> pb.AppendEntriesResponse
	7, // 7: pb.Raft.ForwardApply:output_type -> pb.ApplyResponse
	5, // [5:8] is the sub-list for method output_type
	2, // [2:5] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_raft_proto_init() }
func file_raft_proto_init() {
	if File_raft_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_raft_proto_rawDesc), len(file_raft_proto_rawDesc)),
			NumEnums:      1,
			NumMessages:   7,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_raft_proto_goTypes,
		DependencyIndexes: file_raft_proto_depIdxs,
		EnumInfos:         file_raft_proto_enumTypes,
		MessageInfos:      file_raft_proto_msgTypes,
	}.Build()
	File_raft_proto = out.File
	file_raft_proto_goTypes = nil
	file_raft_proto_depIdxs = nil
}
