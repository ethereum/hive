# Removes all empty keys and values in input.
def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .value != [] and
          .key != null and
          .key != ""
        )
      )
    else .
    end
  )
;

# Converts decimal string to number.
def to_int:
  if . == null then . else .|tonumber end
;

# Converts "1" / "0" to boolean.
def to_bool:
  if . == null then . else
    if . == "1" then true else false end
  end
;

# Replace config in input.
. + {
  "config": {
    "consortium": (if env.HIVE_CONSORTIUM_PERIOD != null and env.HIVE_CONSORTIUM_EPOCH != null then {
        "period": env.HIVE_CONSORTIUM_PERIOD|to_int,
        "epoch": env.HIVE_CONSORTIUM_EPOCH|to_int
    } else null end),
    "chainId": env.HIVE_CHAIN_ID|to_int,
    "homesteadBlock": env.HIVE_FORK_HOMESTEAD|to_int,
    "daoForkBlock": env.HIVE_FORK_DAO_BLOCK|to_int,
    "daoForkSupport": env.HIVE_FORK_DAO_VOTE|to_bool,
    "eip150Block": env.HIVE_FORK_TANGERINE|to_int,
    "eip150Hash": env.HIVE_FORK_TANGERINE_HASH,
    "eip155Block": env.HIVE_FORK_SPURIOUS|to_int,
    "eip158Block": env.HIVE_FORK_SPURIOUS|to_int,
    "byzantiumBlock": env.HIVE_FORK_BYZANTIUM|to_int,
    "constantinopleBlock": env.HIVE_FORK_CONSTANTINOPLE|to_int,
    "odysseusBlock": env.HIVE_FORK_ODYSSEUS|to_int,
    "fenixBlock": env.HIVE_FORK_FENIX|to_int,
    "petersburgBlock": env.HIVE_FORK_PETERSBURG|to_int,
    "istanbulBlock": env.HIVE_FORK_ISTANBUL|to_int,
    "muirGlacierBlock": env.HIVE_FORK_MUIR_GLACIER|to_int,
    "berlinBlock": env.HIVE_FORK_BERLIN|to_int,
    "londonBlock": env.HIVE_FORK_LONDON|to_int,
    "arrowGlacierBlock": env.HIVE_FORK_ARROW_GLACIER|to_int,
    "grayGlacierBlock": env.HIVE_FORK_GRAY_GLACIER|to_int,
    "mergeNetsplitBlock": env.HIVE_MERGE_BLOCK_ID|to_int,
    "shanghaiTime": env.HIVE_SHANGHAI_TIMESTAMP|to_int,
    "cancunTime": env.HIVE_CANCUN_TIMESTAMP|to_int,
    "consortiumV2Block": env.HIVE_FORK_CONSORTIUMV2|to_int,
    "consortiumV2Contracts": (if env.HIVE_RONIN_VALIDATOR_SET != null and env.HIVE_RONIN_SLASH_INDICATOR != null and env.HIVE_RONIN_STAKING_CONTRACT != null then {
        "roninValidatorSet": env.HIVE_RONIN_VALIDATOR_SET,
        "slashIndicator": env.HIVE_RONIN_SLASH_INDICATOR,
        "stakingContract": env.HIVE_RONIN_STAKING_CONTRACT
    } else null end),
  }|remove_empty,
  "alloc": {
      "0x0000000000000000000000000000000000000011": {
          "balance": "0x0",
          "code": "0x608060405234801561001057600080fd5b50600436106100a95760003560e01c80636a0cd1f5116100715780636a0cd1f514610160578063b7ab4db51461018c578063c370b042146101e4578063dafae408146101ec578063facd743b1461021d578063fc81975014610243576100a9565b80630f43a677146100ae57806335aa2e44146100c85780634b561753146101015780634e70b1dc1461012f57806353727d2614610137575b600080fd5b6100b661024b565b60408051918252519081900360200190f35b6100e5600480360360208110156100de57600080fd5b5035610251565b604080516001600160a01b039092168252519081900360200190f35b61012d6004803603604081101561011757600080fd5b50803590602001356001600160a01b0316610278565b005b6100b6610408565b61012d6004803603606081101561014d57600080fd5b508035906020810135906040013561040e565b61012d6004803603604081101561017657600080fd5b50803590602001356001600160a01b03166105a4565b610194610743565b60408051602080825283518183015283519192839290830191858101910280838360005b838110156101d05781810151838201526020016101b8565b505050509050019250505060405180910390f35b6100b66107a5565b6102096004803603602081101561020257600080fd5b50356107ab565b604080519115158252519081900360200190f35b6102096004803603602081101561023357600080fd5b50356001600160a01b03166107e0565b6100e56107fe565b60025481565b6001818154811061025e57fe5b6000918252602090912001546001600160a01b0316905081565b610281336107e0565b61028a57600080fd5b604080516001600160a01b03808416828401526020808301849052600c60608401526b30b2322b30b634b230ba37b960a11b6080808501919091528451808503909101815260a09093019093528151919092012060055490916000911663ec9ab83c6102f461080d565b8685336040518563ffffffff1660e01b81526004018080602001858152602001848152602001836001600160a01b03166001600160a01b03168152602001828103825286818151815260200191508051906020019080838360005b8381101561036757818101518382015260200161034f565b50505050905090810190601f1680156103945780820380516001836020036101000a031916815260200191505b5095505050505050602060405180830381600087803b1580156103b657600080fd5b505af11580156103ca573d6000803e3d6000fd5b505050506040513d60208110156103e057600080fd5b5051905060018160028111156103f257fe5b1415610402576104028484610944565b50505050565b60035481565b610417336107e0565b61042057600080fd5b604080518082018490526060808201849052602080830191909152600c60808301526b75706461746551756f72756d60a01b60a0808401919091528351808403909101815260c090920190925280519101206005546000906001600160a01b031663ec9ab83c61048e61080d565b8785336040518563ffffffff1660e01b81526004018080602001858152602001848152602001836001600160a01b03166001600160a01b03168152602001828103825286818151815260200191508051906020019080838360005b838110156105015781810151838201526020016104e9565b50505050905090810190601f16801561052e5780820380516001836020036101000a031916815260200191505b5095505050505050602060405180830381600087803b15801561055057600080fd5b505af1158015610564573d6000803e3d6000fd5b505050506040513d602081101561057a57600080fd5b50519050600181600281111561058c57fe5b141561059d5761059d858585610a02565b5050505050565b6105ad336107e0565b6105b657600080fd5b6105bf816107e0565b6105c857600080fd5b604080516001600160a01b03808416828401526020808301849052600f60608401526e3932b6b7bb32ab30b634b230ba37b960891b6080808501919091528451808503909101815260a09093019093528151919092012060055490916000911663ec9ab83c61063561080d565b8685336040518563ffffffff1660e01b81526004018080602001858152602001848152602001836001600160a01b03166001600160a01b03168152602001828103825286818151815260200191508051906020019080838360005b838110156106a8578181015183820152602001610690565b50505050905090810190601f1680156106d55780820380516001836020036101000a031916815260200191505b5095505050505050602060405180830381600087803b1580156106f757600080fd5b505af115801561070b573d6000803e3d6000fd5b505050506040513d602081101561072157600080fd5b50519050600181600281111561073357fe5b1415610402576104028484610a69565b6060600180548060200260200160405190810160405280929190818152602001828054801561079b57602002820191906000526020600020905b81546001600160a01b0316815260019091019060200180831161077d575b5050505050905090565b60045481565b60006107c4600254600354610bc890919063ffffffff16565b6004546107d890849063ffffffff610bc816565b101592915050565b6001600160a01b031660009081526020819052604090205460ff1690565b6005546001600160a01b031681565b60055460408051638e46684960e01b815290516060926001600160a01b031691638e466849916004808301926000929190829003018186803b15801561085257600080fd5b505afa158015610866573d6000803e3d6000fd5b505050506040513d6000823e601f3d908101601f19168201604052602081101561088f57600080fd5b81019080805160405193929190846401000000008211156108af57600080fd5b9083019060208201858111156108c457600080fd5b82516401000000008111828201881017156108de57600080fd5b82525081516020918201929091019080838360005b8381101561090b5781810151838201526020016108f3565b50505050905090810190601f1680156109385780820380516001836020036101000a031916815260200191505b50604052505050905090565b6001600160a01b03811660009081526020819052604090205460ff161561096a57600080fd5b6001805480820182557fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b0319166001600160a01b038416908117909155600081815260208190526040808220805460ff191685179055600280549094019093559151909184917f7429a06e9412e469f0d64f9d222640b0af359f556b709e2913588c227851b88d9190a35050565b80821115610a0f57600080fd5b600380546004805492859055839055604080518281526020810184905281519293928592879289927f976f8a9c5bdf8248dec172376d6e2b80a8e3df2f0328e381c6db8e1cf138c0f8929181900390910190a45050505050565b610a72816107e0565b610a7b57600080fd5b6000805b600254811015610acb57826001600160a01b031660018281548110610aa057fe5b6000918252602090912001546001600160a01b03161415610ac357809150610acb565b600101610a7f565b506001600160a01b0382166000908152602081905260409020805460ff1916905560025460018054909160001901908110610b0257fe5b600091825260209091200154600180546001600160a01b039092169183908110610b2857fe5b9060005260206000200160006101000a8154816001600160a01b0302191690836001600160a01b031602179055506001805480610b6157fe5b600082815260208120820160001990810180546001600160a01b03191690559182019092556002805490910190556040516001600160a01b0384169185917f7126bef88d1149ccdff9681ed5aecd3ba5ae70c96517551de250af09cebd1a0b9190a3505050565b600082610bd757506000610bf0565b5081810281838281610be557fe5b0414610bf057600080fd5b9291505056fea265627a7a72315820ee5f68147305f40cf9481c24f13db7dda3a5cbf9c93b41c4ead22f306768974b64736f6c63430005110032",
          "storage": {
              "0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000000000000000000000000000000000000000000002",
              "0x0000000000000000000000000000000000000000000000000000000000000002": "0x0000000000000000000000000000000000000000000000000000000000000002",
              "0x0000000000000000000000000000000000000000000000000000000000000003": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0x0000000000000000000000000000000000000000000000000000000000000004": "0x0000000000000000000000000000000000000000000000000000000000000002",
              "0x0000000000000000000000000000000000000000000000000000000000000005": "0x0000000000000000000000000000000000000000000000000000000000000022",
              "0xe1f02a907819dc15e7569fd211f4b43f42a373aa11f6e990e7f28daa5e307bee": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0xe8cabce05a9d2fe7bd33d67d59df0612177afd050ba66a5d9789ad711208c68b": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6": env.HIVE_VALIDATOR_1_SLOT_VALUE,
              "0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf7": env.HIVE_VALIDATOR_2_SLOT_VALUE
          }
      },
      "0x0000000000000000000000000000000000000022": {
          "balance": "0x0",
          "code": "0x608060405234801561001057600080fd5b50600436106101425760003560e01c80638f283970116100b8578063c7dd11bd1161007c578063c7dd11bd1461067a578063d365a3771461071e578063d87478c3146107bf578063e28d4906146107e8578063ec9ab83c14610805578063f851a440146108bd57610142565b80638f2839701461057d5780639a202d47146105a35780639a307391146105ab5780639f4d9634146105d1578063a07aea1c146105d957610142565b806341433aeb1161010a57806341433aeb146103bd578063435c93da146103ef578063635b27701461046c5780636f91f82f146104925780637284f84d146104c35780638e4668491461057557610142565b806305b2cfbc1461014757806305f41018146101ed57806327f5b35b1461023a5780632f583c89146102e35780633a5381b514610399575b600080fd5b6101eb6004803603602081101561015d57600080fd5b810190602081018135600160201b81111561017757600080fd5b82018360208201111561018957600080fd5b803590602001918460018302840111600160201b831117156101aa57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295506108c5945050505050565b005b6102166004803603606081101561020357600080fd5b508035906020810135906040013561090b565b6040518082600281111561022657fe5b60ff16815260200191505060405180910390f35b6102166004803603606081101561025057600080fd5b810190602081018135600160201b81111561026a57600080fd5b82018360208201111561027c57600080fd5b803590602001918460018302840111600160201b8311171561029d57600080fd5b91908080601f0160208091040260200160405190810160405280939291908181526020018383808284376000920191909152509295505082359350505060200135610931565b610387600480360360208110156102f957600080fd5b810190602081018135600160201b81111561031357600080fd5b82018360208201111561032557600080fd5b803590602001918460018302840111600160201b8311171561034657600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955061096a945050505050565b60408051918252519081900360200190f35b6103a1610985565b604080516001600160a01b039092168252519081900360200190f35b610387600480360360608110156103d357600080fd5b50803590602081013590604001356001600160a01b0316610994565b6103f76109b7565b6040805160208082528351818301528351919283929083019185019080838360005b83811015610431578181015183820152602001610419565b50505050905090810190601f16801561045e5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6101eb6004803603602081101561048257600080fd5b50356001600160a01b03166109e2565b6104af600480360360208110156104a857600080fd5b5035610a1b565b604080519115158252519081900360200190f35b6104af600480360360608110156104d957600080fd5b810190602081018135600160201b8111156104f357600080fd5b82018360208201111561050557600080fd5b803590602001918460018302840111600160201b8311171561052657600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955050823593505050602001356001600160a01b0316610a30565b6103f7610a73565b6101eb6004803603602081101561059357600080fd5b50356001600160a01b0316610aa0565b6101eb610b25565b6104af600480360360208110156105c157600080fd5b50356001600160a01b0316610b84565b6103f7610b99565b6101eb600480360360208110156105ef57600080fd5b810190602081018135600160201b81111561060957600080fd5b82018360208201111561061b57600080fd5b803590602001918460208302840111600160201b8311171561063c57600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600092019190915250929550610bc7945050505050565b6101eb6004803603602081101561069057600080fd5b810190602081018135600160201b8111156106aa57600080fd5b8201836020820111156106bc57600080fd5b803590602001918460018302840111600160201b831117156106dd57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600092019190915250929550610cbe945050505050565b6101eb6004803603602081101561073457600080fd5b810190602081018135600160201b81111561074e57600080fd5b82018360208201111561076057600080fd5b803590602001918460208302840111600160201b8311171561078157600080fd5b919080806020026020016040519081016040528093929190818152602001838360200280828437600092019190915250929550610cfd945050505050565b610387600480360360608110156107d557600080fd5b5080359060208101359060400135610ea5565b6103a1600480360360208110156107fe57600080fd5b5035610ec8565b6102166004803603608081101561081b57600080fd5b810190602081018135600160201b81111561083557600080fd5b82018360208201111561084757600080fd5b803590602001918460018302840111600160201b8311171561086857600080fd5b91908080601f01602080910402602001604051908101604052809392919081815260200183838082843760009201919091525092955050823593505050602081013590604001356001600160a01b0316610eef565b6103a1611123565b6000546001600160a01b031633146108dc57600080fd5b60006108e782611132565b90506108f2816111c4565b6000908152600360205260409020805460ff1916905550565b600660209081526000938452604080852082529284528284209052825290205460ff1681565b60008061093d85611132565b6000908152600660209081526040808320968352958152858220948252939093525050205460ff16919050565b600061097582611132565b9050610980816111c4565b919050565b6007546001600160a01b031681565b600460209081526000938452604080852082529284528284209052825290205481565b6040518060400160405280600f81526020016e11115413d4d25517d0d21053939153608a1b81525081565b6000546001600160a01b031633146109f957600080fd5b600780546001600160a01b0319166001600160a01b0392909216919091179055565b60036020526000908152604090205460ff1681565b600080610a3c85611132565b600090815260046020908152604080832087845282528083206001600160a01b038716845290915290205415159150509392505050565b604051806040016040528060118152602001701590531251105513d497d0d21053939153607a1b81525081565b6000546001600160a01b03163314610ab757600080fd5b6001600160a01b038116610aca57600080fd5b600080546040516001600160a01b03808516939216917f7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f91a3600080546001600160a01b0319166001600160a01b0392909216919091179055565b6000546001600160a01b03163314610b3c57600080fd5b600080546040516001600160a01b03909116917fa3b62bc36326052d97ea62d63c3d60308ed4c3ea8ac079dd8499f1e9c4f80c0f91a2600080546001600160a01b0319169055565b60026020526000908152604090205460ff1681565b6040518060400160405280601281526020017115d2551211149055d05317d0d2105393915360721b81525081565b6000546001600160a01b03163314610bde57600080fd5b6000805b8251811015610cb957828181518110610bf757fe5b6020908102919091018101516001600160a01b0381166000908152600290925260409091205490925060ff16610cb1576001805480820182557fb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf60180546001600160a01b0319166001600160a01b038516908117909155600081815260026020526040808220805460ff1916909417909355915190917fac6fa858e9350a46cec16539926e0fde25b7629f84b5a72bffaae4df888ae86d91a25b600101610be2565b505050565b6000546001600160a01b03163314610cd557600080fd5b6000610ce082611132565b6000908152600360205260409020805460ff191660011790555050565b6000546001600160a01b03163314610d1457600080fd5b6000805b8251811015610dad57828181518110610d2d57fe5b6020908102919091018101516001600160a01b0381166000908152600290925260409091205490925060ff1615610da5576001600160a01b038216600081815260026020526040808220805460ff19169055517f80c0b871b97b595b16a7741c1b06fed0c6f6f558639f18ccbce50724325dc40d9190a25b600101610d18565b5060005b600154811015610cb95760018181548110610dc857fe5b60009182526020808320909101546001600160a01b0316808352600290915260409091205490925060ff16610e9c57600180546000198101908110610e0957fe5b600091825260209091200154600180546001600160a01b039092169183908110610e2f57fe5b600091825260209091200180546001600160a01b0319166001600160a01b0392909216919091179055600180546000198101908110610e6a57fe5b600091825260209091200180546001600160a01b03191690556001805490610e9690600019830161122a565b50610ea0565b6001015b610db1565b600560209081526000938452604080852082529284528284209052825290205481565b60018181548110610ed557fe5b6000918252602090912001546001600160a01b0316905081565b3360009081526002602052604081205460ff16610f0b57600080fd5b6000610f168661096a565b600081815260046020908152604080832089845282528083206001600160a01b038816845290915290205490915015610f805760405162461bcd60e51b815260040180806020018281038252603381526020018061126b6033913960400191505060405180910390fd5b600081815260046020818152604080842089855282528084206001600160a01b038089168652908352818520899055858552600683528185208a86528352818520898652835281852054868652600584528286208b875284528286208a8752845294829020546007548351631b5f5c8160e31b81526001830196810196909652925160ff909616959094929091169263dafae4089260248082019391829003018186803b15801561103057600080fd5b505afa158015611044573d6000803e3d6000fd5b505050506040513d602081101561105a57600080fd5b5051156110d357600082600281111561106f57fe5b14156110a65760008381526006602090815260408083208a845282528083208984529091529020805460ff191660011790556110d3565b60008381526006602090815260408083208a845282528083208984529091529020805460ff191660021790555b5050600081815260056020908152604080832088845282528083208784528252808320805460010190559282526006815282822087835281528282208683529052205460ff169050949350505050565b6000546001600160a01b031681565b6000816040516020018080602001828103825283818151815260200191508051906020019080838360005b8381101561117557818101518382015260200161115d565b50505050905090810190601f1680156111a25780820380516001836020036101000a031916815260200191505b5092505050604051602081830303815290604052805190602001209050919050565b60008181526003602052604090205460ff16611227576040805162461bcd60e51b815260206004820181905260248201527f41636b6e6f776c656467656d656e743a20696e76616c6964206368616e6e656c604482015290519081900360640190fd5b50565b815481835581811115610cb957600083815260209020610cb991810190830161126791905b80821115611263576000815560010161124f565b5090565b9056fe41636b6e6f776c656467656d656e743a207468652076616c696461746f7220616c72656164792061636b6e6f776c6564676564a265627a7a72315820f4d086314cc58377f5b478acafd8e648500c898e2a0c6ed48a4334c07feba0ed64736f6c63430005110032",
          "storage": {
              "0x0000000000000000000000000000000000000000000000000000000000000000": "0x000000000000000000000000968d0cd7343f711216817e617d3f92a23dc91c07",
              "0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0x0000000000000000000000000000000000000000000000000000000000000007": "0x0000000000000000000000000000000000000000000000000000000000000011",
              "0xb10e2d527612073b26eecdfd717e6a320cf44b4afac2b0732d9fcbe2b7fa0cf6": "0x0000000000000000000000000000000000000000000000000000000000000011",
              "0xa4e0f4432e44d027a7b3f953940f096bca7a9bd910297cad2ba7c703c2b799d3": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0xf60e76f92b2a488ed03e0cd79ddc30e3ebf1976f8894261e7f85c346ea7d0f1d": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0x51ecc4de9dec5e56eeda533002a3799c48bd59218c8ba66e6d1b5af56c02e2b5": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0xa6ddbdd89aa33000e6eb92d85fcccf53eb95603a5e73f0d31a88a764062af630": "0x0000000000000000000000000000000000000000000000000000000000000001"
          }
      },
      "0x0000000000000000000000000000000000000033": {
          "balance": "0x0",
          "code": "0x608060405234801561001057600080fd5b50600436106100a5576000357c0100000000000000000000000000000000000000000000000000000000900480639a202d47116100785780639a202d4714610133578063d936547e1461013b578063f59c370814610161578063f851a4401461018f576100a5565b80633af32abf146100aa57806347a3ed0c146100e45780635d54e612146101055780638f2839701461010d575b600080fd5b6100d0600480360360208110156100c057600080fd5b5035600160a060020a03166101b3565b604080519115158252519081900360200190f35b610103600480360360208110156100fa57600080fd5b503515156101e5565b005b6100d0610239565b6101036004803603602081101561012357600080fd5b5035600160a060020a0316610242565b6101036102d6565b6100d06004803603602081101561015157600080fd5b5035600160a060020a0316610342565b6101036004803603604081101561017757600080fd5b50600160a060020a0381351690602001351515610357565b6101976103c2565b60408051600160a060020a039092168252519081900360200190f35b60025460009060ff16806101df5750600160a060020a03821660009081526001602052604090205460ff165b92915050565b600054600160a060020a031633146101fc57600080fd5b6002805460ff19168215159081179091556040517f01d0151a76b6ade3851c417c9a2511eefa3e319bc884bae10c3dc989e7e8d85e90600090a250565b60025460ff1681565b600054600160a060020a0316331461025957600080fd5b600160a060020a038116151561026e57600080fd5b60008054604051600160a060020a03808516939216917f7e644d79422f17c01e4894b5f4f588d331ebfa28653d42ae832dc59e38c9798f91a36000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0392909216919091179055565b600054600160a060020a031633146102ed57600080fd5b60008054604051600160a060020a03909116917fa3b62bc36326052d97ea62d63c3d60308ed4c3ea8ac079dd8499f1e9c4f80c0f91a26000805473ffffffffffffffffffffffffffffffffffffffff19169055565b60016020526000908152604090205460ff1681565b600054600160a060020a0316331461036e57600080fd5b600160a060020a038216600081815260016020526040808220805460ff191685151590811790915590519092917faf367c7d20ce5b2ab6da56afd0c9c39b00ba995263c60292a3e1ee3781fd488591a35050565b600054600160a060020a03168156fea165627a7a723058206965b4ef55d1a0946fb818978e158f73cfa773b0a1397d5e98c3e7a53faf78160029",
          "storage": {
              "0x0000000000000000000000000000000000000000000000000000000000000000": "0x000000000000000000000000968d0cd7343f711216817e617d3f92a23dc91c07",
              "0x0000000000000000000000000000000000000000000000000000000000000002": "0x0000000000000000000000000000000000000000000000000000000000000001",
              "0xf065adf7048bb700993c55970cae3820c9fa96c78b64e20d5de6b7c3c47971f6": "0x0000000000000000000000000000000000000000000000000000000000000001"
          }
      },
      (if env.HIVE_MAIN_ACCOUNT == null then null else env.HIVE_MAIN_ACCOUNT end): {
          "balance": "0x1431e0fae6d7217caa0000000"
      }
  }|remove_empty,
  "coinbase": .coinbase,
  "difficulty": .difficulty,
  "extraData": .extraData,
  "gasLimit": .gasLimit,
  "nonce": .nonce,
  "mixhash": .mixhash,
  "parentHash": .parentHash,
  "timestamp": .timestamp
}