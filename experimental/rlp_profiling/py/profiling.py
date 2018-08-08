import json
import time
import statistics

from ethereum_tx import Transaction as EvmTransaction
from ethereum_utils import decode_hex, parse_int_or_hex, remove_0x_head
from quarkchain_core import Code, MinorBlockHeader, MinorBlock, Transaction


def load_block(file_path: str):
    with open(file_path) as f:
        data = json.load(f)

    header = MinorBlockHeader(
        height=1,
        hashPrevMinorBlock=bytes.fromhex(data["env"]["previousHash"][2:]),
        createTime=int(data["env"]["currentTimestamp"], 16),
        difficulty=int(data["env"]["currentDifficulty"], 16),
    )

    txlist = []
    for txdata in data["transactions"]:
        evm_tx = EvmTransaction(
            nonce=parse_int_or_hex(txdata["nonce"] or b"0"),
            gasprice=parse_int_or_hex(txdata["gasPrice"] or b"0"),
            startgas=parse_int_or_hex(txdata["gasLimit"][0] or b"0"),
            to=decode_hex(remove_0x_head(txdata["to"])),
            value=parse_int_or_hex(txdata["value"][0] or b"0"),
            data=decode_hex(remove_0x_head(txdata["data"][0])),
        )
        evm_tx.sign(decode_hex(remove_0x_head(txdata["secretKey"])))
        tx = Transaction(code=Code.createEvmCode(evm_tx))
        txlist.append(tx)

    return MinorBlock(header, txlist)


if __name__ == "__main__":
    block = load_block("../BlockData.json")
    block_bytes = block.serialize()
    print("byte length: %d" % len(block_bytes))

    warmup_runs, measure_runs = [500, 10]

    # Serialization performance.
    for _ in range(warmup_runs):
        block.serialize()
    time_list = []
    for _ in range(measure_runs):
        start = time.time()
        block.serialize()
        time_list.append(int((time.time() - start) * 1e6))
    mean, stdev = statistics.mean(time_list), statistics.stdev(time_list)
    print("profiling serialization, mean: %f, stdev: %f" % (mean, stdev))

    # Deserialization performance.
    for _ in range(warmup_runs):
        recovered_block = MinorBlock.deserialize(block_bytes)
        assert len(recovered_block.txList) == len(block.txList)
    time_list = []
    for _ in range(measure_runs):
        start = time.time()
        MinorBlock.deserialize(block_bytes)
        time_list.append(int((time.time() - start) * 1e6))
    mean, stdev = statistics.mean(time_list), statistics.stdev(time_list)
    print("profiling deserialization, mean: %f, stdev: %f" % (mean, stdev))
