#!/usr/bin/env python3
"""Character error rate for the issue #35 OCR evaluation set.

Usage: python3 -I score_ocr.py <ground-truth.txt> <ocr-output.txt>

Both texts are NFKC-normalised and all whitespace is removed before the
Levenshtein distance is divided by the ground-truth length. An empty ground
truth scores 0.0 only when the output is also empty; otherwise 1.0.
"""
import sys
import unicodedata


def normalise(text: str) -> str:
    return "".join(unicodedata.normalize("NFKC", text).split())


def distance(a: str, b: str) -> int:
    previous = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        current = [i]
        for j, cb in enumerate(b, 1):
            current.append(min(previous[j] + 1, current[j - 1] + 1, previous[j - 1] + (ca != cb)))
        previous = current
    return previous[-1]


def cer(truth: str, output: str) -> float:
    truth, output = normalise(truth), normalise(output)
    if not truth:
        return 0.0 if not output else 1.0
    return distance(truth, output) / len(truth)


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    with open(sys.argv[1], encoding="utf-8") as t, open(sys.argv[2], encoding="utf-8") as o:
        print(f"{cer(t.read(), o.read()):.4f}")
