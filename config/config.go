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

package config

import "github.com/satori/go.uuid"
import "github.com/deroproject/derohe/cryptography/crypto"

// all global configuration variables are picked from here

// though testing has complete successfully with 3 secs block time, however
// consider homeusers/developing countries we will be targetting  9 secs
// later hardforks can make it lower by 1 sec, say every 6 months or so, until the system reaches 3 secs
// by that time, networking,space requirements  and  processing requiremtn will probably outgrow homeusers
// since most mining nodes will be running in datacenter, 3 secs  blocks c
const BLOCK_TIME = uint64(18)

// note we are keeping the tree name small for disk savings, since they will be stored n times (atleast or archival nodes)
// this is used by graviton
const BALANCE_TREE = "B" // keeps main balance
const SC_META = "M"      // keeps all SCs balance, their state, their OWNER, their data tree top hash is stored here
// one are open SCs, which provide i/o privacy
// one are private SCs which are truly private, in which no one has visibility of io or functionality

// 1.25 MB block every 12 secs is equal to roughly 75 TX per second
// if we consider side blocks, TPS increase to > 100 TPS
// we can easily improve TPS by changing few parameters in this file
// the resources compute/network may not be easy for the developing countries
// we need to trade of TPS  as per community
const STARGATE_HE_MAX_BLOCK_SIZE = uint64((1 * 1024 * 1024) + (256 * 1024)) // max block size limit

const STARGATE_HE_MAX_TX_SIZE = 300 * 1024 // max size

const MIN_MIXIN = 2   //  >= 2 ,   mixin will be accepted
const MAX_MIXIN = 128 // <= 128,  mixin will be accepted

// ATLANTIS FEE calculation constants are here
const FEE_PER_KB = uint64(1000000000) // .001 dero per kb

const MAINNET_BOOTSTRAP_DIFFICULTY = uint64(800 * BLOCK_TIME) // atlantis mainnet botstrapped at 200 MH/s
const MAINNET_MINIMUM_DIFFICULTY = uint64(800 * BLOCK_TIME)   // 5 KH/s

// testnet bootstraps at 1 MH
//const  TESTNET_BOOTSTRAP_DIFFICULTY = uint64(1000*1000*BLOCK_TIME)
const TESTNET_BOOTSTRAP_DIFFICULTY = uint64(800 * BLOCK_TIME) // testnet bootstrap at 800 H/s
const TESTNET_MINIMUM_DIFFICULTY = uint64(800 * BLOCK_TIME)   // 800 H


//this controls the batch size which controls till how many blocks incoming funds cannot be spend
const BLOCK_BATCH_SIZE = crypto.BLOCK_BATCH_SIZE 

// this single parameter controls lots of various parameters
// within the consensus, it should never go below 7
// if changed responsibly, we can have one second  or lower blocks (ignoring chain bloat/size issues)
// gives immense scalability,
const STABLE_LIMIT = int64(8)


// we can have number of chains running for testing reasons
type CHAIN_CONFIG struct {
	Name       string
	Network_ID uuid.UUID // network ID

	P2P_Default_Port        int
	RPC_Default_Port        int
	Wallet_RPC_Default_Port int

	Dev_Address string // to which address the dev's share of fees must go

	Genesis_Nonce uint32

	Genesis_Block_Hash crypto.Hash

	Genesis_Tx string
}

var Mainnet = CHAIN_CONFIG{Name: "mainnet",
	Network_ID:              uuid.FromBytesOrNil([]byte{0x59, 0xd7, 0xf7, 0xe9, 0xdd, 0x48, 0xd5, 0xfd, 0x13, 0x0a, 0xf6, 0xe0, 0x9a, 0x44, 0x45, 0x0}),
	P2P_Default_Port:        10101,
	RPC_Default_Port:        10102,
	Wallet_RPC_Default_Port: 10103,
	Genesis_Nonce:           10000,

	Genesis_Block_Hash: crypto.HashHexToHash("e14e318562db8d22f8d00bd41c7938807c7ff70e4380acc6f7f2427cf49f474a"),

	Genesis_Tx: "" +
		"01" + // version
		"00" + // PREMINE_FLAG
		"8fff7f" + // PREMINE_VALUE
		"a01f9bcc1208dee302769931ad378a4c0c4b2c21b0cfb3e752607e12d2b6fa6425", // miners public key

	Dev_Address: "deto1qxsplx7vzgydacczw6vnrtfh3fxqcjevyxcvlvl82fs8uykjkmaxgfgulfha5",
}

var Testnet = CHAIN_CONFIG{Name: "testnet", // testnet will always have last 3 bytes 0
	Network_ID:              uuid.FromBytesOrNil([]byte{0x59, 0xd7, 0xf7, 0xe9, 0xdd, 0x48, 0xd5, 0xfd, 0x13, 0x0a, 0xf6, 0xe0, 0x26, 0x00, 0x02, 0x00}),
	P2P_Default_Port:        40401,
	RPC_Default_Port:        40402,
	Wallet_RPC_Default_Port: 40403,
	Genesis_Nonce:           10000,

	Genesis_Block_Hash: crypto.HashHexToHash("7be4a8f27bcadf556132dba38c2d3d78214beec8a959be17caf172317122927a"),

	Genesis_Tx: "" +
		"01" + // version
		"00" + // PREMINE_FLAG
		"8fff7f" + // PREMINE_VALUE
		"a01f9bcc1208dee302769931ad378a4c0c4b2c21b0cfb3e752607e12d2b6fa6425", // miners public key

	Dev_Address: "deto1qxsplx7vzgydacczw6vnrtfh3fxqcjevyxcvlvl82fs8uykjkmaxgfgulfha5",
}

// mainnet has a remote daemon node, which can be used be default, if user provides a  --remote flag
const REMOTE_DAEMON = "https://rwallet.dero.live"
