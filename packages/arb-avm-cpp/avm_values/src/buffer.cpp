/*
 * Copyright 2019-2020, Offchain Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

#include <avm_values/buffer.hpp>
#include <avm_values/bigint.hpp>

#include <ethash/keccak.hpp>

uint256_t zero_hash(int sz) {
    if (sz == 32) {
        return hash(0);
    }
    auto h1 = zero_hash(sz/2);
    return hash(h1, h1);
}

Packed normal(uint256_t hash, int sz) {
    return Packed{hash, sz, 0};
}

Packed pack(const Packed& packed) {
    return Packed{packed.hash, packed.size, packed.packed+1};
}

bool is_zero_hash(const Packed& packed) {
    return packed.hash == hash(0);
}

uint256_t unpack(const Packed &packed) {
    uint256_t res = packed.hash;
    int sz = packed.size;
    for (int i = 0; i < packed.packed; i++) {
        res = hash(res, zero_hash(sz));
        sz = sz*2;
    }
    return res;
}

Packed zero_packed(int sz) {
    if (sz == 32) {
        return normal(zero_hash(32), 32);
    }
    return pack(zero_packed(sz/2));
}

Packed hash_buf(uint8_t *buf, int offset, int sz) {
    if (sz == 32) {
        auto hash_val = ethash::keccak256(buf+offset, 32);
        uint256_t res = intx::be::load<uint256_t>(hash_val);
        return normal(res, 32);
    }
    auto h1 = hash_buf(buf, offset, sz/2);
    auto h2 = hash_buf(buf, offset+sz/2, sz/2);
    if (is_zero_hash(h2)) {
        return pack(h1);
    }
    return normal(hash(unpack(h1), unpack(h2)), sz);
}

Packed hash_node(Buffer *buf, int offset, int len, int sz) {
    if (len == 1) {
        return buf[0].hash_aux();
    }
    auto h1 = hash_node(buf, offset, len/2, sz/2);
    auto h2 = hash_node(buf, offset + len/2, len/2, sz/2);
    if (is_zero_hash(h2)) {
        return pack(h1);
    }
    return normal(hash(unpack(h1), unpack(h2)), sz);
}

uint256_t Buffer::hash() const {
    return hash_aux().hash;
}

Packed Buffer::hash_aux() const {
    if (level == 0) {
        if (!leaf) {
            return zero_packed(1024);
        }
        return hash_buf(leaf->data(), 0, 1024);
    } else {
        if (!leaf) {
            return zero_packed(calc_len(level));
        }
        return hash_node(node->data(), 0, 128, calc_len(level));
    }
}

uint256_t hash(const Buffer& b) {
    return b.hash();
}

