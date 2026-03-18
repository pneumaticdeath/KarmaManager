#!/usr/bin/env python3

import json
import sys

with open("avoid.json", "r") as a:
    avoid = set(json.loads(a.read()))

for f in sys.argv[1:]:
    print(f"Processing {f}")
    with open(f, "r") as fh:
        d = list(json.loads(fh.read()))
    filtered = []
    for word in d:
        if word not in avoid:
            filtered.append(word)

    with open(f, "w") as ofh:
        ofh.write(json.dumps(filtered, indent=4))
