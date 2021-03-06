// CST_Server project main.go
package HandleRequest

import (
	"Server/DbUitl"
	"Server/FIFOQueue"
	"Server/MyProbuf"
	"Server/OptUtil"
	"Server/data"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/golang/protobuf/proto"
)

const (
	msg_length = 2 * 1024
)

var socketMap = make(map[int]net.Conn)

type ClientSrc struct {
	msg      *DataFrame.Msg
	send_msg *DataFrame.Msg
	char_msg *data.Message
	msgIndex int
}

func StratServer() {
	log.Println("start to listen...")
	listen, err := net.Listen("tcp", ":6666")
	if err != nil {
		fmt.Println("listen error:", err)
	}
	defer listen.Close()
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("accept error:", err)
			continue
		}
		log.Println("connected from " + conn.RemoteAddr().String())
		client_src := &ClientSrc{
			msg:      &DataFrame.Msg{},
			send_msg: &DataFrame.Msg{},
			char_msg: &data.Message{},
			msgIndex: 0,
		}
		go handleConn(conn, client_src)
	}
}

func handleConn(conn net.Conn, this *ClientSrc) {
	DbUitl.ConnectDb()
	data := make([]byte, msg_length)
	for {
		n, err := conn.Read(data)
		if err != nil {
			fmt.Println(conn.RemoteAddr().String() + "退出")
			break
		}
		err = proto.Unmarshal(data[0:n], this.msg)
		if err != nil {
			fmt.Println("unmarshaling error: ", err)
		}
		switch this.msg.UserOpt {
		case OptUtil.REQUEST_LOGIN:
			this.handleLogin(conn)
			break
		case OptUtil.REQUEST_GET_FRIENDS:
			this.handleGetFriend(conn)
			break
		case OptUtil.REQUEST_SEND_TXT:
			this.handleSendText(conn)
			break
		case OptUtil.REQUEST_GET_OFFLINE_MSG:
			this.handleGetOffLineMsg(conn)
			break
		case OptUtil.REQUEST_EXIT:
			this.handleExit(conn)
			break

		}
	}
	fmt.Println("msg:", this.msg)
	defer conn.Close()
}

func (this *ClientSrc) handleLogin(conn net.Conn) {
	username := this.msg.User.UesrName
	password := this.msg.User.UserPwd
	fmt.Println("username:" + username)
	fmt.Println("password:" + password)
	var userdata = data.User{}
	if username != "" && password != "" {
		userdata = DbUitl.Login(username, password)
		fmt.Println("Server:", userdata)
	}
	if userdata.UserId != 0 {
		fmt.Println("send success...")
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_LOGIN,
			OptResult:     true,
			ReceiveResult: "Success",
			User: &DataFrame.User{
				UserID:   int32(userdata.UserId),
				UesrName: userdata.UserName,
				NickName: userdata.NickeName,
			},
		}
	} else {
		fmt.Println("send faild...")
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_LOGIN,
			OptResult:     false,
			ReceiveResult: "UserName or Password Eorro..",
		}
	}
	socketMap[userdata.UserId] = conn
	data, err := proto.Marshal(this.send_msg)
	fmt.Println("Marshal:", this.send_msg)
	if err != nil {
		fmt.Println("marshaling error: ", err)
	}
	conn.Write(data)
	fmt.Println("Write data....", data)
}

func (this *ClientSrc) handleGetFriend(conn net.Conn) {
	useId := int(this.msg.User.UserID)
	fmt.Println("recive userid:", useId)
	friendMap := make(map[int]data.User)
	var index int
	if useId != 0 {
		index, friendMap = DbUitl.GetFriends(strconv.Itoa(useId))
		fmt.Println("friendMap:", friendMap)
	}
	friendlist := make([]*DataFrame.User, index)
	if friendMap != nil {
		var listIndex int = 0
		for _, value := range friendMap {
			fmt.Println("value:", value)
			friends := new(DataFrame.User)
			friends.UserID = int32(value.UserId)
			friends.UesrName = value.UserName
			friends.NickName = value.NickeName
			friendlist[listIndex] = friends
			listIndex++
		}
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_GET_FRIEND,
			OptResult:     true,
			ReceiveResult: "Success",
			Friends:       friendlist,
		}
	} else {
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_GET_FRIEND,
			OptResult:     false,
			ReceiveResult: "get friends fail...",
		}
	}
	data, err := proto.Marshal(this.send_msg)
	fmt.Println("Marshal:", this.send_msg)
	if err != nil {
		fmt.Println("marshaling error: ", err)
	}
	conn.Write(data)
	fmt.Println("friend send_msg:", this.send_msg)

}

func (this *ClientSrc) handleSendText(conn net.Conn) {
	index := this.msgIndex
	this.char_msg.ReciverID = int(this.msg.PersonalMsg[index].RecverID)
	this.char_msg.SenderID = int(this.msg.PersonalMsg[index].SenderID)
	this.char_msg.Content = this.msg.PersonalMsg[index].Content
	this.char_msg.Time = this.msg.PersonalMsg[index].SendTime
	this.char_msg.DataType = int(OptUtil.MESSAGE_TYPE_TXT)
	fmt.Println("handleSendText:", this.char_msg)
	_, ok := socketMap[this.char_msg.ReciverID]
	if ok {
		conn := socketMap[this.char_msg.ReciverID]
		msgList := make([]*DataFrame.PersonalMsg, 1)
		msgList = this.msg.PersonalMsg
		message := &DataFrame.Msg{
			UserOpt:       OptUtil.RECEIVE_TEXT,
			OptResult:     true,
			ReceiveResult: "success",
			PersonalMsg:   msgList,
		}
		data, err := proto.Marshal(message)
		fmt.Println("Marshal:", message)
		if err != nil {
			fmt.Println("marshaling error: ", err)
		}
		conn.Write(data)
		fmt.Println("handle send_msg:", message)
	} else {
		DbUitl.SaveMessage(this.char_msg)
		fmt.Println("用户" + strconv.Itoa(this.char_msg.ReciverID) + "不在线，先把消息暂存在服务器端")
	}
	this.msgIndex++
}

func (this *ClientSrc) handleGetOffLineMsg(conn net.Conn) {
	fmt.Println("handleGetOffLineMsg()....")
	userId := int(this.msg.User.UserID)
	msgQueue := new(FIFOQueue.Queue)
	if userId != 0 {
		msgQueue = DbUitl.GetOffLineMsg(strconv.Itoa(userId))
	}
	size := msgQueue.Size()
	fmt.Println("msgQueue size:", size)
	DbUitl.DeleteMessage(strconv.Itoa(userId))
	msgList := make([]*DataFrame.PersonalMsg, size)
	var listIndex int = 0
	if msgQueue.Size() != 0 {
		for msgQueue.Size() > 0 {
			msg_data := msgQueue.Dequeue().Value.(*DataFrame.PersonalMsg)
			fmt.Println(msg_data)
			msgList[listIndex] = msg_data
			listIndex++
		}
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_GET_OFFLINE_MSG,
			OptResult:     true,
			ReceiveResult: "Success",
			PersonalMsg:   msgList,
		}
	} else {
		this.send_msg = &DataFrame.Msg{
			UserOpt:       OptUtil.RESULT_GET_OFFLINE_MSG,
			OptResult:     true,
			ReceiveResult: "No OffLine Message",
		}
	}

	data, err := proto.Marshal(this.send_msg)
	fmt.Println("Marshal:", this.send_msg)
	if err != nil {
		fmt.Println("marshaling error: ", err)
	}
	conn.Write(data)
	fmt.Println("handleGetOffline send_msg:", this.send_msg)

}

func (this *ClientSrc) handleExit(conn net.Conn) {
	userId := int(this.msg.User.UserID)
	fmt.Println("在线用户", userId, "请求退出登录")
	conn.Close()
	delete(socketMap, userId)
	fmt.Println("用户", userId, "退出前,共有", len(socketMap), "个用户在线")
	DbUitl.Close()
}

func BytesToInt(b []byte) int {
	bytesBuffer := bytes.NewBuffer(b)
	var x int32
	binary.Read(bytesBuffer, binary.BigEndian, &x)
	return int(x)

}

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}
