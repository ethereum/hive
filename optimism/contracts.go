package optimism

import "github.com/ethereum/go-ethereum/common"

type HardhatDeployment struct {
	Address common.Address `json:"address"`
	Abi     []struct {
		Inputs []struct {
			InternalType string `json:"internalType"`
			Name         string `json:"name"`
			Type         string `json:"type"`
			Indexed      bool   `json:"indexed,omitempty"`
		} `json:"inputs,omitempty"`
		StateMutability string `json:"stateMutability,omitempty"`
		Type            string `json:"type"`
		Anonymous       bool   `json:"anonymous,omitempty"`
		Name            string `json:"name,omitempty"`
		Outputs         []struct {
			InternalType string `json:"internalType"`
			Name         string `json:"name"`
			Type         string `json:"type"`
		} `json:"outputs,omitempty"`
	} `json:"abi"`
	TransactionHash common.Hash `json:"transactionHash"`
	Receipt         struct {
		To               *common.Address `json:"to"`
		From             common.Address  `json:"from"`
		ContractAddress  common.Address  `json:"contractAddress"`
		TransactionIndex int             `json:"transactionIndex"`
		GasUsed          string          `json:"gasUsed"`
		LogsBloom        string          `json:"logsBloom"`
		BlockHash        string          `json:"blockHash"`
		TransactionHash  string          `json:"transactionHash"`
		Logs             []struct {
			TransactionIndex int      `json:"transactionIndex"`
			BlockNumber      int      `json:"blockNumber"`
			TransactionHash  string   `json:"transactionHash"`
			Address          string   `json:"address"`
			Topics           []string `json:"topics"`
			Data             string   `json:"data"`
			LogIndex         int      `json:"logIndex"`
			BlockHash        string   `json:"blockHash"`
		} `json:"logs"`
		BlockNumber       int    `json:"blockNumber"`
		CumulativeGasUsed string `json:"cumulativeGasUsed"`
		Status            int    `json:"status"`
		Byzantium         bool   `json:"byzantium"`
	} `json:"receipt"`
	Args             []string `json:"args"`
	NumDeployments   int      `json:"numDeployments"`
	SolcInputHash    string   `json:"solcInputHash"`
	Metadata         string   `json:"metadata"`
	Bytecode         string   `json:"bytecode"`
	DeployedBytecode string   `json:"deployedBytecode"`
	Devdoc           struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Methods struct {
			Admin struct {
				Returns struct {
					Field1 string `json:"_0"`
				} `json:"returns"`
			} `json:"admin()"`
			ChangeAdminAddress struct {
				Params struct {
					Admin string `json:"_admin"`
				} `json:"params"`
			} `json:"changeAdmin(address)"`
			Constructor struct {
				Params struct {
					Admin string `json:"_admin"`
				} `json:"params"`
			} `json:"constructor"`
			Implementation struct {
				Returns struct {
					Field1 string `json:"_0"`
				} `json:"returns"`
			} `json:"implementation()"`
			UpgradeToAddress struct {
				Params struct {
					Implementation string `json:"_implementation"`
				} `json:"params"`
			} `json:"upgradeTo(address)"`
			UpgradeToAndCallAddressBytes struct {
				Params struct {
					Data           string `json:"_data"`
					Implementation string `json:"_implementation"`
				} `json:"params"`
			} `json:"upgradeToAndCall(address,bytes)"`
		} `json:"methods"`
		Events struct {
			AdminChangedAddressAddress struct {
				Params struct {
					NewAdmin      string `json:"newAdmin"`
					PreviousAdmin string `json:"previousAdmin"`
				} `json:"params"`
			} `json:"AdminChanged(address,address)"`
			UpgradedAddress struct {
				Params struct {
					Implementation string `json:"implementation"`
				} `json:"params"`
			} `json:"Upgraded(address)"`
		} `json:"events"`
		Title string `json:"title"`
	} `json:"devdoc"`
	Userdoc struct {
		Version int    `json:"version"`
		Kind    string `json:"kind"`
		Methods struct {
			Admin struct {
				Notice string `json:"notice"`
			} `json:"admin()"`
			ChangeAdminAddress struct {
				Notice string `json:"notice"`
			} `json:"changeAdmin(address)"`
			Constructor struct {
				Notice string `json:"notice"`
			} `json:"constructor"`
			Implementation struct {
				Notice string `json:"notice"`
			} `json:"implementation()"`
			UpgradeToAddress struct {
				Notice string `json:"notice"`
			} `json:"upgradeTo(address)"`
			UpgradeToAndCallAddressBytes struct {
				Notice string `json:"notice"`
			} `json:"upgradeToAndCall(address,bytes)"`
		} `json:"methods"`
		Events struct {
			AdminChangedAddressAddress struct {
				Notice string `json:"notice"`
			} `json:"AdminChanged(address,address)"`
			UpgradedAddress struct {
				Notice string `json:"notice"`
			} `json:"Upgraded(address)"`
		} `json:"events"`
		Notice string `json:"notice"`
	} `json:"userdoc"`
}

type HardhatDeploymentsL1 struct {
	L1CrossDomainMessengerProxy HardhatDeployment `json:"L1CrossDomainMessengerProxy"`
	L1StandardBridgeProxy       HardhatDeployment `json:"L1StandardBridgeProxy"`
	L2OutputOracleProxy         HardhatDeployment `json:"L2OutputOracleProxy"`
	OptimismPortalProxy         HardhatDeployment `json:"OptimismPortalProxy"`
}

type DeploymentsL1 struct {
	L1CrossDomainMessengerProxy common.Address
	L1StandardBridgeProxy       common.Address
	L2OutputOracleProxy         common.Address
	OptimismPortalProxy         common.Address
}

type DeploymentsL2 struct {
	L1Block common.Address
}

type Deployments struct {
	DeploymentsL1
	DeploymentsL2
}
