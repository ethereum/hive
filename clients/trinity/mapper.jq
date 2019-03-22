def remove_empty:
  . | walk(
    if type == "object" then
      with_entries(
        select(
          .value != null and
          .value != "" and
          .value != [] and
          .value != {}
        )
      )
    else .
    end
  );

def addashex:
  if . != null  and startswith("0x") then . else 
    if . !=null then "0x"+. else . end
  end
;

def infixzerostolength(s;l):
   if . != null then
     (.[0:s])+("0"*(l-(.|length)))+(.[s:l])
   else .
   end
;


.|
.alloc|=with_entries(.key|="0x"+.) |
{"accounts": .alloc,
  "genesis": {
    "author":.coinbase,
    "difficulty":.difficulty,
    "extraData":.extraData|infixzerostolength(2;66),
    "gasLimit":.gasLimit,
    "nonce": .nonce,
    "timestamp":.timestamp,
    
  },
 	"params": {
     
    "DAOForkBlock": env.HIVE_FORK_DAO_BLOCK|addashex,
		"EIP150ForkBlock": env.HIVE_FORK_TANGERINE|addashex,
		"EIP158ForkBlock": env.HIVE_FORK_SPURIOUS|addashex,
		"byzantiumForkBlock": env.HIVE_FORK_BYZANTIUM|addashex,
		"homesteadForkBlock": env.HIVE_FORK_HOMESTEAD|addashex,
		"miningMethod": (if env.HIVE_SKIP_POW!=null then "ethash" else "NoProof" end)
   },
  
	"version": "1"
}|remove_empty










  