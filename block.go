package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
)

type Block struct {
	Hash         []byte
	Transactions []*Transaction
	PreviousHash []byte
	Nonce        int
}

func CreateBlock(transactions []*Transaction, previousHash []byte) *Block {
	block := &Block{[]byte{}, transactions, previousHash, 0}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce
	return block
}

func (block *Block) HashTransactions() []byte {
	var txHashes [][]byte
	var txHash [32]byte
	for _, tx := range block.Transactions {
		txHashes = append(txHashes, tx.Id)
	}
	txHash = sha256.Sum256(bytes.Join(txHashes, []byte{}))
	return txHash[:]
}

func FatherBlock(coinbase *Transaction) *Block {
	return CreateBlock([]*Transaction{coinbase}, []byte{})
}

func (block *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(block)
	Handle(err)
	return res.Bytes()
}

func Deserialize(data []byte) *Block {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)
	Handle(err)
	return &block
}
