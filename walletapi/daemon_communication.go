// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package walletapi

// this file needs  serious improvements but have extremely limited time
/* this file handles communication with the daemon
 * this includes receiving output information
 *
 * *
 */
//import "io"
//import "os"
import "fmt"
import "time"
import "sync"
import "bytes"
import "math/big"

//import "bufio"
import "strings"
import "context"

//import "runtime"
//import "compress/gzip"
import "encoding/hex"

import "runtime/debug"

import "github.com/romana/rlog"

//import "github.com/vmihailenco/msgpack"

//import "github.com/gorilla/websocket"
//import "github.com/mafredri/cdp/rpcc"

import "github.com/deroproject/derohe/rpc"
import "github.com/deroproject/derohe/block"
import "github.com/deroproject/derohe/cryptography/crypto"
import "github.com/deroproject/derohe/errormsg"
import "github.com/deroproject/derohe/transaction"
import "github.com/deroproject/derohe/cryptography/bn256"

import "github.com/creachadair/jrpc2"

// this global variable should be within wallet structure
var Connected bool = false

// there should be no global variables, so multiple wallets can run at the same time with different assset

var endpoint string

var output_lock sync.Mutex

var NotifyNewBlock *sync.Cond = sync.NewCond(&sync.Mutex{})
var NotifyHeightChange *sync.Cond = sync.NewCond(&sync.Mutex{})

// this function will wait n goroutines to wait for new block
func WaitNewBlock() {
	NotifyNewBlock.L.Lock()
	NotifyNewBlock.Wait()
	NotifyNewBlock.L.Unlock()
}

// this function will wait n goroutines to wait  till height changes
func WaitNewHeightBlock() {
	NotifyHeightChange.L.Lock()
	NotifyHeightChange.Wait()
	NotifyHeightChange.L.Unlock()
}

func Notify_broadcaster(req *jrpc2.Request) {

	timer.Reset(timeout) // connection is alive
	switch req.Method() {

	case "Repoll":
		NotifyNewBlock.L.Lock()
		NotifyNewBlock.Broadcast()
		NotifyNewBlock.L.Unlock()
	case "HRepoll":
		NotifyHeightChange.L.Lock()
		NotifyHeightChange.Broadcast()
		NotifyHeightChange.L.Unlock()
	default:
		rlog.Debugf("Notification received %s\n", req.Method())
	}

}

// triggers syncing with wallet every 5 seconds
func (w *Wallet_Memory) sync_loop() {
	w.Sync_Wallet_Memory_With_Daemon() // sync with the daemon
}

func (cli *Client) Call(method string, params interface{}, result interface{}) error {
	return cli.RPC.CallResult(context.Background(), method, params, result)
}

// returns whether wallet was online some time ago
func (w *Wallet_Memory) IsDaemonOnlineCached() bool {
	return Connected
}

// currently process url  with compatibility for older ip address
func buildurl(endpoint string) string {
	if strings.IndexAny(endpoint, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") >= 0 { // url is already complete
		return strings.TrimSuffix(endpoint, "/")
	} else {
		return "http://" + endpoint
	}
}

// this is as simple as it gets
// single threaded communication to get the daemon status and height
// this will tell whether the wallet can connection successfully to  daemon or not
func IsDaemonOnline() bool {
	if rpc_client.WS == nil || rpc_client.RPC == nil {
		return false
	}
	return true
}

// sync the wallet with daemon, this is instantaneous and can be done with a single call
// we have now the apis to avoid polling
func (w *Wallet_Memory) Sync_Wallet_Memory_With_Daemon() {
	first_time := true
	for {
		select {
		case <-w.Quit:
			break
		default:

		}

		if !IsDaemonOnline() {
			w.Daemon_Height = 0
			w.Daemon_TopoHeight = 0
		}

		if (first_time && IsDaemonOnline()) || (!first_time && IsDaemonOnline()) {
			first_time = false
			//w.random_ring_members()
			rlog.Debugf("wallet topo height %d daemon online topo height %d\n", w.account.TopoHeight, w.Daemon_TopoHeight)
			previous := w.account.Balance_Result.Data
			var scid crypto.Hash
			if _, _,_,_,_, err := w.GetEncryptedBalanceAtTopoHeight(scid, -1, w.GetAddress().String()); err == nil {
				if w.account.Balance_Result.Data != previous /*|| (len(w.account.EntriesNative[scid]) >= 1 && strings.ToLower(w.account.Balance_Result.Data) != strings.ToLower(w.account.EntriesNative[scid][len(w.account.EntriesNative[scid])-1].EWData)) */ {
					w.DecodeEncryptedBalance() // try to decode balance

					w.SyncHistory(scid) // also update statement
					w.save_if_disk()    // save wallet
				} else {
					w.save_if_disk() // save wallet
				}
			} else {
				rlog.Infof("getbalance err %s", err)
			}
		}
		time.Sleep(timeout) // wait 5 seconds
	}
	return
}

// this is as simple as it gets
// single threaded communication to relay TX to daemon
// if this is successful, then daemon is in control

func (w *Wallet_Memory) SendTransaction(tx *transaction.Transaction) (err error) {

	if tx == nil {
		return fmt.Errorf("Can not send nil transaction")
	}

	if !IsDaemonOnline() {
		return fmt.Errorf("offline or not connected. cannot send transaction.")
	}

	params := rpc.SendRawTransaction_Params{Tx_as_hex: hex.EncodeToString(tx.Serialize())}
	var result rpc.SendRawTransaction_Result

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.SendRawTransaction", params, &result); err != nil {
		return err
	}

	//fmt.Printf("raw transaction result %+v\n", result)

	if result.Status == "OK" {
		return nil
	} else {
		err = fmt.Errorf("Err %s", result.Status)
	}

	//fmt.Printf("err in response %+v", result)

	return
}

func (w *Wallet_Memory) DecodeEncryptedBalance() (err error) {
	hexdecoded, err := hex.DecodeString(w.account.Balance_Result.Data)
	if err != nil {
		return
	}

	el := new(crypto.ElGamal).Deserialize(hexdecoded)
	if err != nil {
		panic(err)
		return
	}

	w.account.Balance_Mature = w.DecodeEncryptedBalanceNow(el)
	return nil
}

// decode encrypted balance now
// it may take a long time, its currently sing threaded, need to parallelize
func (w *Wallet_Memory) DecodeEncryptedBalanceNow(el *crypto.ElGamal) uint64 {

	balance_point := new(bn256.G1).Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))
	return Balance_lookup_table.Lookup(balance_point, w.account.Balance_Mature)
}

// this is as simple as it gets
// single threaded communication  gets whether the the key image is spent in pool or in blockchain
// this can leak informtion which keyimage belongs to us
// TODO in order to stop privacy leaks we must guess this information somehow on client side itself
// maybe the server can broadcast a bloomfilter or something else from the mempool keyimages
//
func (w *Wallet_Memory) GetEncryptedBalanceAtTopoHeight(scid crypto.Hash, topoheight int64, accountaddr string) (bits int, e *crypto.ElGamal, height,rtopoheight uint64, merkleroot crypto.Hash, err error) {

	defer func() {
		if r := recover(); r != nil {
			rlog.Warnf("Stack trace  \n%s", debug.Stack())

		}
	}()

	if !w.GetMode() { // if wallet is in offline mode , we cannot do anything
		err = fmt.Errorf("wallet is in offline mode")
		return
	}

	if !IsDaemonOnline() {
		err = fmt.Errorf("offline or not connected")
		return
	}

	//var params rpc.GetEncryptedBalance_Params
	var result rpc.GetEncryptedBalance_Result

	// Issue a call with a response.
	if err = rpc_client.Call("DERO.GetEncryptedBalance", rpc.GetEncryptedBalance_Params{SCID: scid, Address: accountaddr, TopoHeight: topoheight}, &result); err != nil {
		rlog.Errorf("Call failed: %v", err)

		if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(errormsg.ErrAccountUnregistered.Error())) && accountaddr == w.GetAddress().String() && scid.IsZero() {
			w.Error = errormsg.ErrAccountUnregistered
		}

		//fmt.Printf("will return errr now \n")

		// all SCID users are considered registered and their balance is assumed zero

		if !scid.IsZero() {
			if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(errormsg.ErrAccountUnregistered.Error())) {
				if addr, err1 := rpc.NewAddress(accountaddr); err1 != nil {
					err = err1
					return
				} else {
					e = crypto.ConstructElGamal(addr.PublicKey.G1(), crypto.ElGamal_BASE_G) // init zero balance
					bits = 128
					err = nil
					return
				}
			}
		}
		return
	}

	//	fmt.Printf("GetEncryptedBalance result  %+v\n", result)
	if scid.IsZero() && accountaddr == w.GetAddress().String() {
		if result.Status == errormsg.ErrAccountUnregistered.Error() {
			w.Error = errormsg.ErrAccountUnregistered
			w.account.Registered = false
		} else {
			w.account.Registered = true
		}
	}

//		fmt.Printf("status '%s' err '%s'  %+v  %+v \n", result.Status , w.Error , result.Status == errormsg.ErrAccountUnregistered.Error()  , accountaddr == w.account.GetAddress().String())

	if scid.IsZero() && result.Status == errormsg.ErrAccountUnregistered.Error() {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	w.Daemon_Height = uint64(result.DHeight)
	w.Daemon_TopoHeight = result.DTopoheight
	w.Merkle_Balance_TreeHash = result.DMerkle_Balance_TreeHash

	if topoheight == -1 && scid.IsZero() && accountaddr == w.GetAddress().String() {
		w.account.Balance_Result = result
		w.account.TopoHeight = result.Topoheight
	}

	if scid.IsZero() && result.Status != "OK" {
		err = fmt.Errorf("%s", result.Status)
		return
	}

	hexdecoded, err := hex.DecodeString(result.Data)
	if err != nil {
		return
	}

	if accountaddr == w.GetAddress().String() && scid.IsZero() {
		w.Error = nil
	}

	el := new(crypto.ElGamal).Deserialize(hexdecoded)

	hexdecoded, err = hex.DecodeString(result.Merkle_Balance_TreeHash)
	if err != nil {
		return
	}

	var mhash crypto.Hash
	copy(mhash[:],hexdecoded[:])

	return result.Bits, el, uint64(result.Height), uint64(result.Topoheight), mhash, nil
}

func (w *Wallet_Memory) DecodeEncryptedBalance_Memory(el *crypto.ElGamal, hint uint64) (balance uint64) {

	var balance_point bn256.G1

	balance_point.Add(el.Left, new(bn256.G1).Neg(new(bn256.G1).ScalarMult(el.Right, w.account.Keys.Secret.BigInt())))

	return Balance_lookup_table.Lookup(&balance_point, hint)
}

func (w *Wallet_Memory) GetDecryptedBalanceAtTopoHeight(scid crypto.Hash, topoheight int64, accountaddr string) (balance uint64, err error) {
	_, encrypted_balance,_,_,_, err := w.GetEncryptedBalanceAtTopoHeight(scid, topoheight, accountaddr)
	if err != nil {
		return 0, err
	}

	return w.DecodeEncryptedBalance_Memory(encrypted_balance, 0), nil
}

// sync history of wallet from blockchain
func (w *Wallet_Memory) random_ring_members(scid crypto.Hash) (alist []string) {

	//fmt.Printf("getting random_ring_members\n")

	//if len(w.account.RingMembers) > 300 { // unregistered so skip
	//	return
	//}

	var result rpc.GetRandomAddress_Result

	// Issue a call with a response.
	if err := rpc_client.Call("DERO.GetRandomAddress", rpc.GetRandomAddress_Params{SCID: scid}, &result); err != nil {
		rlog.Errorf("GetRandomAddress Call failed: %v", err)
		return
	}
	//fmt.Printf("ring members %+v\n", result)

	// we have found a matching block hash, start syncing from here
	//if w.account.RingMembers == nil {
	////	w.account.RingMembers = map[string]int64{}
	//}

	for _, k := range result.Address {
		if k != w.GetAddress().String() {
			alist = append(alist, k)
		}
	}
	return
}

// sync history of wallet from blockchain
func (w *Wallet_Memory) SyncHistory(scid crypto.Hash) (balance uint64) {
	if w.account.Balance_Result.Registration < 0 { // unregistered so skip
		return
	}

	last_topo_height := int64(-1)

	//fmt.Printf("finding sync point  ( Registration point %d)\n", w.account.Balance_Result.Registration)

	entries := w.account.EntriesNative[scid]

	// we need to find a sync point, to minimize traffic
	for i := len(entries) - 1; i >= 0; {

		// below condition will trigger if chain got pruned on server
		if w.account.Balance_Result.Registration >= entries[i].TopoHeight { // keep old history if chain got pruned
			break
		}
		if last_topo_height == entries[i].TopoHeight {
			i--
		} else {

			last_topo_height = entries[i].TopoHeight

			var result rpc.GetBlockHeaderByHeight_Result

			// Issue a call with a response.
			if err := rpc_client.Call("DERO.GetBlockHeaderByTopoHeight", rpc.GetBlockHeaderByTopoHeight_Params{TopoHeight: uint64(entries[i].TopoHeight)}, &result); err != nil {
				rlog.Errorf("GetBlockHeaderByTopoHeight Call failed: %v", err)
				return 0
			}

			if i >= 1 && last_topo_height == entries[i-1].TopoHeight { // skipping any entries withing same block
				for ; i >= 1; i-- {
					if last_topo_height != entries[i-1].TopoHeight {
						entries = entries[:i]
						w.account.EntriesNative[scid] = entries
					}
				}
			}

			if i == 0 {
				w.account.EntriesNative[scid] = entries[:0] // discard all entries
				break
			}

			// we have found a matching block hash, start syncing from here
			if result.Status == "OK" && result.Block_Header.Hash == entries[i].BlockHash {
				w.synchistory_internal(scid, entries[i].TopoHeight+1, w.account.Balance_Result.Topoheight)
				return
			}

		}

	}

	//fmt.Printf("syncing loop using Registration %d\n", w.account.Balance_Result.Registration)

	// if we reached here, means we should sync from scratch
	w.synchistory_internal(scid, w.account.Balance_Result.Registration, w.account.Balance_Result.Topoheight)

	//if w.account.Registration >= 0 {
	// err :=
	// err =  w.synchistory_internal(w.account.Registration,6)

	// }
	// fmt.Printf("syncing err %s\n",err)
	// fmt.Printf("entries %+v\n", w.account.Entries)

	return 0
}

// sync history
func (w *Wallet_Memory) synchistory_internal(scid crypto.Hash, start_topo, end_topo int64) error {

	var err error
	var start_balance_e *crypto.ElGamal
	if start_topo == w.account.Balance_Result.Registration {
		start_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		_, start_balance_e,_,_,_, err = w.GetEncryptedBalanceAtTopoHeight(scid, start_topo, w.GetAddress().String())
		if err != nil {
			return err
		}
	}

	_, end_balance_e,_,_,_, err := w.GetEncryptedBalanceAtTopoHeight(scid, end_topo, w.GetAddress().String())
	if err != nil {
		return err
	}

	return w.synchistory_internal_binary_search(scid, start_topo, start_balance_e, end_topo, end_balance_e)

}

func (w *Wallet_Memory) synchistory_internal_binary_search(scid crypto.Hash, start_topo int64, start_balance_e *crypto.ElGamal, end_topo int64, end_balance_e *crypto.ElGamal) error {

	//fmt.Printf("end %d start %d\n", end_topo, start_topo)

	if end_topo < 0 {
		return fmt.Errorf("done")
	}

	/*	if bytes.Compare(start_balance_e.Serialize(), end_balance_e.Serialize()) == 0 {
		    return nil
		}
	*/

	//for start_topo <= end_topo{
	{
		median := (start_topo + end_topo) / 2

		//fmt.Printf("low %d high %d median %d\n", start_topo,end_topo,median)

		if start_topo == median {
			//fmt.Printf("syncing block %d\n", start_topo)
			err := w.synchistory_block(scid, start_topo)
			if err != nil {
				return err
			}
		}

		if end_topo-start_topo <= 1 {
			return w.synchistory_block(scid, end_topo)
		}

		_, median_balance_e,_,_,_, err := w.GetEncryptedBalanceAtTopoHeight(scid, median, w.GetAddress().String())
		if err != nil {
			return err
		}

		// check if there is a change in lower section, if yes process more
		if start_topo == w.account.Balance_Result.Registration || bytes.Compare(start_balance_e.Serialize(), median_balance_e.Serialize()) != 0 {
			//fmt.Printf("lower\n")
			err = w.synchistory_internal_binary_search(scid, start_topo, start_balance_e, median, median_balance_e)
			if err != nil {
				return err
			}
		}

		// check if there is a change in higher section, if yes process more
		if bytes.Compare(median_balance_e.Serialize(), end_balance_e.Serialize()) != 0 {
			//fmt.Printf("higher\n")
			err = w.synchistory_internal_binary_search(scid, median, median_balance_e, end_topo, end_balance_e)
			if err != nil {
				return err
			}
		}

		/*if IsRegisteredAtTopoHeight (addr,median) {
		            high = median - 1
				}else{
					low = median + 1
				}*/

	}

	/*
	       if end_topo - start_topo <= 1 {
	   		err :=  w.synchistory_block(start_topo)
	   		if err != nil {
	   			return err
	   		}
	   		return w.synchistory_block(end_topo)
	   	}


	       // this means the address is either a ring member or a sender or a receiver in atleast one of the blocks
	       middle :=  start_topo +  (end_topo-start_topo)/2
	       middle_balance_e, err := w.GetEncryptedBalanceAtTopoHeight( middle ,w.account.GetAddress().String())
	   	if err != nil {
	   		return err
	   	}

	   	// check if there is a change in lower section, if yes process more
	   	if bytes.Compare(start_balance_e.Serialize(), middle_balance_e.Serialize()) != 0 {
	   		fmt.Printf("lower\n")
	           err = w.synchistory_internal_binary_search(start_topo,start_balance_e, middle, middle_balance_e )
	           if err != nil {
	           	return err
	           }
	       }

	       // check if there is a change in lower section, if yes process more
	       if bytes.Compare(middle_balance_e.Serialize(), end_balance_e.Serialize()) != 0 {
	       	fmt.Printf("higher\n")
	           err = w.synchistory_internal_binary_search(middle, middle_balance_e, end_topo,end_balance_e )
	           if err != nil {
	           	return err
	           }
	       }
	*/

	return nil
}

// extract history from a single block
// first get a block, then get all the txs
// Todo we should expose an API to get all txs which have the specific address as ring member
// for a particular block
// for the entire chain
func (w *Wallet_Memory) synchistory_block(scid crypto.Hash, topo int64) (err error) {

	var local_entries []rpc.Entry

	compressed_address := w.account.Keys.Public.EncodeCompressed()

	var previous_balance_e, current_balance_e *crypto.ElGamal
	var previous_balance, current_balance, total_sent, total_received uint64

	if topo <= 0 || w.account.Balance_Result.Registration == topo {
		previous_balance_e = crypto.ConstructElGamal(w.account.Keys.Public.G1(), crypto.ElGamal_BASE_G)
	} else {
		_, previous_balance_e,_,_,_, err = w.GetEncryptedBalanceAtTopoHeight(scid, topo-1, w.GetAddress().String())
		if err != nil {
			return err
		}
	}

	_, current_balance_e,_,_,_, err = w.GetEncryptedBalanceAtTopoHeight(scid, topo, w.GetAddress().String())
	if err != nil {
		return err
	}

	EWData := fmt.Sprintf("%x", current_balance_e.Serialize())

	previous_balance = w.DecodeEncryptedBalance_Memory(previous_balance_e, 0)
	current_balance = w.DecodeEncryptedBalance_Memory(current_balance_e, 0)

	// we can skip some check if both balances are equal ( means we are ring members in this block)
	// this check will also fail if we total spend == total receivein the block
	// currently it is not implmented, and we bruteforce everything

	_ = current_balance

	var bl block.Block
	var bresult rpc.GetBlock_Result
	if err = rpc_client.Call("DERO.GetBlock", rpc.GetBlock_Params{Height: uint64(topo)}, &bresult); err != nil {
		return fmt.Errorf("getblock rpc failed")
	}

	if bresult.Block_Header.SideBlock { // skip side blocks
		return nil
	}

	block_bin, _ := hex.DecodeString(bresult.Blob)
	bl.Deserialize(block_bin)

	if len(bl.Tx_hashes) >= 1 {

		//fmt.Printf("Requesting tx data %s", txhash);

		for i := range bl.Tx_hashes {
			var tx transaction.Transaction

			var tx_params rpc.GetTransaction_Params
			var tx_result rpc.GetTransaction_Result

			tx_params.Tx_Hashes = append(tx_params.Tx_Hashes, bl.Tx_hashes[i].String())

			if err = rpc_client.Call("DERO.GetTransaction", tx_params, &tx_result); err != nil {
				return fmt.Errorf("gettransa rpc failed %s", err)
			}

			tx_bin, err := hex.DecodeString(tx_result.Txs_as_hex[0])
			if err != nil {
				return err
			}
			if err = tx.DeserializeHeader(tx_bin); err != nil {
				rlog.Warnf("Error deserialing txid %s incoming bytes '%s'", bl.Tx_hashes[i].String(), tx_result.Txs_as_hex[0])
				continue
			}

			if tx.TransactionType == transaction.REGISTRATION {
				continue
			}

			// since balance might change with tx, we track within tx using this
			previous_balance_e_tx := new(crypto.ElGamal).Deserialize(previous_balance_e.Serialize())

			for t := range tx.Payloads {
				if int(tx.Payloads[t].Statement.RingSize) != len(tx_result.Txs[0].Ring[t]) {
					rlog.Warnf("Error expected %d ringmembers for  txid %s  but got %d ", int(tx.Payloads[t].Statement.RingSize), bl.Tx_hashes[i].String(), len(tx_result.Txs[t].Ring))
					continue
				}

				if !tx.Payloads[t].SCID.IsZero() { // skip private tokens for now
					continue
				}

				previous_balance = w.DecodeEncryptedBalanceNow(previous_balance_e_tx)

				for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ { // first fill in all the ring members
					var buf [33]byte
					copy(buf[:], tx_result.Txs[0].Ring[t][j][:])
					tx.Payloads[t].Statement.Publickeylist_compressed = append(tx.Payloads[t].Statement.Publickeylist_compressed, buf)
				}

				for j := 0; j < int(tx.Payloads[t].Statement.RingSize); j++ { // check whether statement has public key

					// check whether our address is a ring member if yes, process it as ours
					if bytes.Compare(compressed_address, tx.Payloads[t].Statement.Publickeylist_compressed[j][:]) == 0 {

						// this tx contains us either as a ring member, or sender or receiver, so add all  members as ring members for future
						// keep collecting ring members to make things exponentially complex

						for k := range tx.Payloads[t].Statement.Publickeylist_compressed {
							var p bn256.G1
							if err = p.DecodeCompressed(tx.Payloads[t].Statement.Publickeylist_compressed[k][:]); err != nil {
								fmt.Printf("key could not be decompressed")

							} else {
								tx.Payloads[t].Statement.Publickeylist = append(tx.Payloads[t].Statement.Publickeylist, &p)
							}
						}

						/*for k := range tx.Statement.Publickeylist_compressed {
							if j != k {
								ringmember := address.NewAddressFromKeys((*crypto.Point)(tx.Statement.Publickeylist[k]))
								ringmember.Mainnet = w.GetNetwork()
								w.account.RingMembers[ringmember.String()] = 1
							}
						}*/

						changes := crypto.ConstructElGamal(tx.Payloads[t].Statement.C[j], tx.Payloads[t].Statement.D)
						changed_balance_e := previous_balance_e_tx.Add(changes)

						previous_balance_e_tx = new(crypto.ElGamal).Deserialize(changed_balance_e.Serialize())

						changed_balance := w.DecodeEncryptedBalance_Memory(changed_balance_e, previous_balance)

						//fmt.Printf("%d changed_balance %d previous_balance %d len payload %d\n", t, changed_balance, previous_balance, len(tx.Payloads[t].RPCPayload))

						entry := rpc.Entry{Height: bl.Height, Pos: t, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: i, TXID: tx.GetHash().String(), Time: time.Unix(int64(bl.Timestamp), 0), Fees: tx.Fees()}

						entry.EWData = EWData
						ring_member := false

						switch {
						case previous_balance == changed_balance: //ring member/* handle 0 value tx but fees is deducted */
							//fmt.Printf("Anon Ring Member in TX %s\n", bl.Tx_hashes[i].String())
							ring_member = true
						case previous_balance > changed_balance: // we generated this tx
							entry.Burn = tx.Payloads[t].BurnValue
							entry.Amount = previous_balance - changed_balance - (tx.Payloads[t].Statement.Fees)
							entry.Fees = tx.Payloads[t].Statement.Fees
							entry.Status = 1                        // mark it as spend
							total_sent += entry.Amount + entry.Fees // burn is in amount

							rinputs := append([]byte{}, tx.Payloads[t].Statement.Roothash[:]...)
							for l := range tx.Payloads[t].Statement.Publickeylist_compressed {
								rinputs = append(rinputs, tx.Payloads[t].Statement.Publickeylist_compressed[l][:]...)
							}
							rencrypted := new(bn256.G1).ScalarMult(crypto.HashToPoint(crypto.HashtoNumber(append([]byte(crypto.PROTOCOL_CONSTANT), rinputs...))), w.account.Keys.Secret.BigInt())
							r := crypto.ReducedHash(rencrypted.EncodeCompressed())

							//fmt.Printf("t %d r  calculated %s\n", t, r.Text(16))
							// lets separate ring members

							for k := range tx.Payloads[t].Statement.C {
								// skip self address, this can be optimized way more
								if tx.Payloads[t].Statement.Publickeylist[k].String() != w.account.Keys.Public.G1().String() {
									var x bn256.G1
									x.ScalarMult(crypto.G, new(big.Int).SetInt64(int64(entry.Amount-entry.Burn)))
									x.Add(new(bn256.G1).Set(&x), new(bn256.G1).ScalarMult(tx.Payloads[t].Statement.Publickeylist[k], r))
									if x.String() == tx.Payloads[t].Statement.C[k].String() {
										// lets encrypt the payment id, it's simple, we XOR the paymentID
										blinder := new(bn256.G1).ScalarMult(tx.Payloads[t].Statement.Publickeylist[k], r)

										// proof is blinder + amount transferred, it will recover the encrypted rpc payload also also
										proof := rpc.NewAddressFromKeys((*crypto.Point)(blinder))
										proof.Proof = true
										proof.Arguments = rpc.Arguments{{rpc.RPC_VALUE_TRANSFER, rpc.DataUint64, uint64(entry.Amount - entry.Burn)}}
										entry.Proof = proof.String()

										entry.PayloadType = tx.Payloads[t].RPCType
										switch tx.Payloads[t].RPCType {

										case 0:
											crypto.EncryptDecryptUserData(blinder, tx.Payloads[t].RPCPayload)
											sender_idx := uint(tx.Payloads[t].RPCPayload[0])

											if sender_idx <= uint(tx.Payloads[t].Statement.RingSize) {
												addr := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[sender_idx]))
												addr.Mainnet = w.GetNetwork()
												entry.Sender = addr.String()
											}

											entry.Payload = append(entry.Payload, tx.Payloads[t].RPCPayload[1:]...)
											entry.Data = append(entry.Data, tx.Payloads[t].RPCPayload[:]...)

											args, _ := entry.ProcessPayload()
											_ = args

										//	fmt.Printf("data received %s idx %d arguments %s\n", string(entry.Payload), sender_idx, args)

										default:
											entry.PayloadError = fmt.Sprintf("unknown payload type %d", tx.Payloads[t].RPCType)
											entry.Payload = tx.Payloads[t].RPCPayload
										}

										//paymentID := binary.BigEndian.Uint64(payment_id_encrypted_bytes[:]) // get decrypted payment id

										addr := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[k]))
										addr.Mainnet = w.GetNetwork()

										entry.Destination = addr.String()

										//fmt.Printf("%d Sent funds to %s entry %+v\n", tx.Height, addr.String(), entry)
										break

									}

								}
							}

						case previous_balance < changed_balance: // someone sentus this amount
							entry.Amount = changed_balance - previous_balance
							entry.Incoming = true

							// we should decode the payment id
							var x bn256.G1
							x.ScalarMult(crypto.G, new(big.Int).SetInt64(0-int64(entry.Amount))) // increase receiver's balance
							x.Add(new(bn256.G1).Set(&x), tx.Payloads[t].Statement.C[j])          // get the blinder

							blinder := &x

							// enable receiver side proofs
							proof := rpc.NewAddressFromKeys((*crypto.Point)(blinder))
							proof.Proof = true
							proof.Arguments = rpc.Arguments{{rpc.RPC_VALUE_TRANSFER, rpc.DataUint64, uint64(entry.Amount)}}
							entry.Proof = proof.String()

							entry.PayloadType = tx.Payloads[t].RPCType
							switch tx.Payloads[t].RPCType {

							case 0:
								crypto.EncryptDecryptUserData(blinder, tx.Payloads[t].RPCPayload)
								sender_idx := uint(tx.Payloads[t].RPCPayload[0])

								if sender_idx <= uint(tx.Payloads[t].Statement.RingSize) {
									addr := rpc.NewAddressFromKeys((*crypto.Point)(tx.Payloads[t].Statement.Publickeylist[sender_idx]))
									addr.Mainnet = w.GetNetwork()
									entry.Sender = addr.String()
								}

								entry.Payload = append(entry.Payload, tx.Payloads[t].RPCPayload[1:]...)
								entry.Data = append(entry.Data, tx.Payloads[t].RPCPayload[:]...)

								args, _ := entry.ProcessPayload()
								_ = args

							//	fmt.Printf("data received %s idx %d arguments %s\n", string(entry.Payload), sender_idx, args)

							default:
								entry.PayloadError = fmt.Sprintf("unknown payload type %d", tx.Payloads[t].RPCType)
								entry.Payload = tx.Payloads[t].RPCPayload
							}

							//fmt.Printf("Received %s amount in TX(%d) %s payment id %x Src_ID %s data %s\n", globals.FormatMoney(changed_balance-previous_balance), tx.Height, bl.Tx_hashes[i].String(),  entry.PaymentID, tx.Src_ID, tx.Data)
							//fmt.Printf("Received  amount in TX(%d) %s payment id %x Src_ID %s data %s\n",  tx.Height, bl.Tx_hashes[i].String(),  entry.PaymentID, tx.SrcID, tx.Data)
							total_received += (changed_balance - previous_balance)
						}

						if !ring_member { // do not book keep ring members
							local_entries = append(local_entries, entry)
						}

						//break // this tx has been processed so skip it

					}
				}
			}

			//fmt.Printf("block %d   %+v\n", topo, tx_result)
		}
	}

	previous_balance = w.DecodeEncryptedBalance_Memory(previous_balance_e, 0)
	coinbase_reward := current_balance - (previous_balance - total_sent + total_received)

	//fmt.Printf("ht %d coinbase_reward %d   curent balance %d previous_balance %d sent %d received %d\n", bl.Height, coinbase_reward, current_balance, previous_balance, total_sent, total_received)

	if bytes.Compare(compressed_address, bl.Miner_TX.MinerAddress[:]) == 0 || coinbase_reward > 0 { // wallet user  has minted a block
		entry := rpc.Entry{Height: bl.Height, TopoHeight: topo, BlockHash: bl.GetHash().String(), TransactionPos: -1, Time: time.Unix(int64(bl.Timestamp), 0)}

		entry.EWData = EWData
		entry.Amount = current_balance - (previous_balance - total_sent + total_received)
		entry.Coinbase = true
		local_entries = append([]rpc.Entry{entry}, local_entries...)

		//fmt.Printf("Coinbase Reward %s for block %d\n", globals.FormatMoney(current_balance-(previous_balance-total_sent+total_received)), topo)
	}

	for _, e := range local_entries {
		w.InsertReplace(scid, e)
	}

	if len(local_entries) >= 1 {
		w.save_if_disk() // save wallet()
		//	w.db.Sync()
	}

	return nil

}
