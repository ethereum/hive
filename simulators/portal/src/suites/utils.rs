// Execution Layer hard forks https://ethereum.org/en/history/
pub const CANCUN_BLOCK_NUMBER: u64 = 19426587;
pub const SHANGHAI_BLOCK_NUMBER: u64 = 17034870;
pub const MERGE_BLOCK_NUMBER: u64 = 15537394;
pub const LONDON_BLOCK_NUMBER: u64 = 12965000;
pub const BERLIN_BLOCK_NUMBER: u64 = 12244000;
pub const ISTANBUL_BLOCK_NUMBER: u64 = 9069000;
pub const CONSTANTINOPLE_BLOCK_NUMBER: u64 = 7280000;
pub const BYZANTIUM_BLOCK_NUMBER: u64 = 4370000;
pub const HOMESTEAD_BLOCK_NUMBER: u64 = 1150000;

pub fn get_flair(block_number: u64) -> String {
    if block_number > CANCUN_BLOCK_NUMBER {
        " (post-cancun)".to_string()
    } else if block_number > SHANGHAI_BLOCK_NUMBER {
        " (post-shanghai)".to_string()
    } else if block_number > MERGE_BLOCK_NUMBER {
        " (post-merge)".to_string()
    } else if block_number > LONDON_BLOCK_NUMBER {
        " (post-london)".to_string()
    } else if block_number > BERLIN_BLOCK_NUMBER {
        " (post-berlin)".to_string()
    } else if block_number > ISTANBUL_BLOCK_NUMBER {
        " (post-istanbul)".to_string()
    } else if block_number > CONSTANTINOPLE_BLOCK_NUMBER {
        " (post-constantinople)".to_string()
    } else if block_number > BYZANTIUM_BLOCK_NUMBER {
        " (post-byzantium)".to_string()
    } else if block_number > HOMESTEAD_BLOCK_NUMBER {
        " (post-homestead)".to_string()
    } else {
        "".to_string()
    }
}
