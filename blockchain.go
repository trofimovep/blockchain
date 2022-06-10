package main

import (
	"encoding/hex"
	"fmt"
	badger "github.com/dgraph-io/badger/v3"
	"os"
	"runtime"
)

const (
	dbPath          = "./tmp/blocks"
	dbFile          = "./tmp/blocks/MANIFEST"
	fatherBlockData = "First Transaction from Father Block"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func (chain *BlockChain) AddBlock(transactions []*Transaction) {
	var lastHash []byte
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		lastHash, err = item.ValueCopy([]byte("lh"))
		return err
	})
	Handle(err)
	newBlock := CreateBlock(transactions, lastHash)
	err = chain.Database.Update(func(txn *badger.Txn) error {
		err = txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), newBlock.Hash)
		chain.LastHash = newBlock.Hash
		return err
	})
	Handle(err)
}

func ContinueBlockChain(address string) *BlockChain {
	if DBExists() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}
	var lastHash []byte

	opts := badger.DefaultOptions(dbPath)

	db, err := badger.Open(opts)
	Handle(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		lastHash, err = item.ValueCopy([]byte("lh"))
		return err
	})
	Handle(err)
	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) FindUnspentTransactions(address string) []Transaction {
	var unspentTxs []Transaction
	spentTxs := make(map[string][]int)

	iter := chain.Iterator()
	for {
		block := iter.Next()
		for _, tx := range block.Transactions {
			txId := hex.EncodeToString(tx.Id)
		Outputs:
			for idx, out := range tx.Outputs {
				if spentTxs[txId] != nil {
					for _, spentOut := range spentTxs[txId] {
						if spentOut == idx {
							continue Outputs
						}
					}
				}
				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}
			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					if in.CanUnlock(address) {
						inTxID := hex.EncodeToString(in.Id)
						spentTxs[inTxID] = append(spentTxs[inTxID], in.Out)
					}
				}
			}
		}
		if len(block.PreviousHash) == 0 {
			break
		}
	}
	return unspentTxs
}

func (chain *BlockChain) FindUTXO(address string) []TxOutput {
	var UTXOs []TxOutput
	unspentTransactions := chain.FindUnspentTransactions(address)
	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}
	return UTXOs
}

func (chain *BlockChain) FindSpendableOutputs(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTxs := chain.FindUnspentTransactions(address)
	accumulated := 0
Work:
	for _, tx := range unspentTxs {
		txID := hex.EncodeToString(tx.Id)
		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIdx)

				if accumulated >= amount {
					break Work
				}
			}
		}
	}
	return accumulated, unspentOuts
}

func InitBlockChain(address string) *BlockChain {
	var lastHash []byte
	if DBExists() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}
	opts := badger.DefaultOptions(dbPath)
	db, err := badger.Open(opts)
	Handle(err)
	err = db.Update(func(txn *badger.Txn) error {
		coinbaseTx := CoinbaseTx(address, fatherBlockData)
		fatherBlock := FatherBlock(coinbaseTx)
		fmt.Println("Father block created!")
		err = txn.Set(fatherBlock.Hash, fatherBlock.Serialize())
		Handle(err)
		err = txn.Set([]byte("lh"), fatherBlock.Hash)
		lastHash = fatherBlock.Hash
		return err
	})
	Handle(err)
	return &BlockChain{lastHash, db}
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	return &BlockChainIterator{chain.LastHash, chain.Database}
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)
		encodedBlock, err := item.ValueCopy(iter.CurrentHash)
		block = Deserialize(encodedBlock)
		return err
	})
	Handle(err)
	iter.CurrentHash = block.PreviousHash
	return block
}

func DBExists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}
