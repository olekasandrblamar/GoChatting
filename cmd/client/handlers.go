package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io/ioutil"
	"log"

	"github.com/haakonleg/go-e2ee-chat-engine/util"
	"github.com/haakonleg/go-e2ee-chat-engine/websock"
)

func savePrivKey(privKey *rsa.PrivateKey) {
	pem := util.MarshalPrivate(privKey)
	if err := ioutil.WriteFile(privKeyFile, pem, 0644); err != nil {
		log.Fatal(err)
	}
}

// Called when user pressed the "create user" button
func (c *Client) createUserHandler(server string, username string) {
	if !c.Connect(server) {
		return
	}

	// Generate new key pair
	privKey, pubKey := util.GenKeyPair()

	// Send a request to register the user
	regUserMsg := &websock.RegisterUserMessage{
		Username:  username,
		PublicKey: util.MarshalPublic(pubKey)}

	websock.SendMessage(c.sock, websock.RegisterUser, regUserMsg, websock.JSON)

	res, err := websock.GetResponse(c.sock)
	if err != nil {
		c.gui.ShowDialog("Did not get a response from the server")
		return
	}

	if res.Type == websock.MessageOK {
		// Save private key to file
		savePrivKey(privKey)
	}

	c.gui.ShowDialog("User created. You can now log in.")
}

// Called when the user pressed the "login user" button
// TODO: Refactor the huge function
func (c *Client) loginUserHandler(server string, username string) {
	if !c.Connect(server) {
		return
	}

	// Read private key from file
	pem, err := ioutil.ReadFile(privKeyFile)
	if err != nil {
		c.gui.ShowDialog("Error reading privatekey.pem file")
		return
	}

	privKey, err := util.UnmarshalPrivate(pem)
	if err != nil {
		c.gui.ShowDialog("Error parsing private key")
		return
	}

	// Send log in request to server
	websock.SendMessage(c.sock, websock.LoginUser, username, websock.String)

	// Recieve auth challenge from server
	res, err := websock.GetResponse(c.sock)
	if err != nil {
		c.gui.ShowDialog("Error receiving auth challenge from server")
		return
	} else if res.Type == websock.Error {
		c.gui.ShowDialog(string(res.Message))
		return
	}

	// Try to decrypt auth challenge
	decKey, err := rsa.DecryptPKCS1v15(rand.Reader, privKey, res.Message)
	if err != nil {
		c.gui.ShowDialog("Invalid private key")
		return
	}

	// Send decrypted auth key to server
	websock.SendMessage(c.sock, websock.AuthChallengeResponse, decKey, websock.Bytes)

	// Check response from server
	if res, err = websock.GetResponse(c.sock); err != nil || res.Type != websock.MessageOK {
		c.gui.ShowDialog("Invalid private key")
		return
	}

	// Login success, show the chat rooms GUI
	c.privateKey = privKey
	c.authKey = decKey
	c.gui.ShowChatRoomGUI(c)
}

func (c *Client) createRoomHandler(name string) {
	// Send request to create new chat room to server
	req := &websock.CreateChatRoomMessage{
		Name:    name,
		AuthKey: c.authKey}

	websock.SendMessage(c.sock, websock.CreateChatRoom, req, websock.JSON)
}

func (c *Client) getChatRooms() *websock.GetChatRoomsResponseMessage {
	// Send request for chat rooms
	websock.SendMessage(c.sock, websock.GetChatRooms, nil, websock.Nil)

	// Get chat rooms response from server
	res, err := websock.GetResponse(c.sock)
	if err != nil {
		c.gui.ShowDialog(err.Error())
		return nil
	}

	// Unmarshal response
	chatRoomsResponse := new(websock.GetChatRoomsResponseMessage)
	if err := json.Unmarshal(res.Message, chatRoomsResponse); err != nil {
		c.gui.ShowDialog("Error parsing chat rooms response")
		return nil
	}
	return chatRoomsResponse
}

func (c *Client) joinChatHandler(name string) {
	// Send request to join chat room
	req := &websock.JoinChatMessage{
		Name:    name,
		AuthKey: c.authKey}

	websock.SendMessage(c.sock, websock.JoinChat, req, websock.JSON)

	if _, err := websock.GetResponse(c.sock); err != nil {
		c.gui.ShowDialog(err.Error())
	}

	// Show the chat interface
	c.gui.ShowChatGUI(c)
}
