#!/usr/bin/env python3
"""
Generate bar charts from Go benchmark output (benchmarks/results/current.txt).
Outputs: assertion_chart.png, runner_chart.png, hooks_chart.png in benchmarks/results/.
Run from repository root.
"""
import re
import sys
from pathlib import Path
from collections import defaultdict

try:
    import matplotlib
    matplotlib.use("Agg")
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
except ImportError:
    print("matplotlib is required: pip install matplotlib", file=sys.stderr)
    sys.exit(1)

# Default paths (run from repo root)
REPO_ROOT = Path(__file__).resolve().parent.parent
RESULTS_DIR = REPO_ROOT / "benchmarks" / "results"
DEFAULT_INPUT = RESULTS_DIR / "current.txt"

# Go benchmark line: BenchmarkName-N   Niter   X.XX ns/op  ...
BENCH_LINE = re.compile(
    r"^Benchmark([A-Za-z0-9_]+)-\d+\s+\d+\s+([\d.]+)\s+ns/op"
)


def parse_benchmarks(path: Path) -> dict[str, list[float]]:
    """Parse current.txt; return map of benchmark_base_name -> list of ns/op (one per line)."""
    by_name: dict[str, list[float]] = defaultdict(list)
    with open(path, encoding="utf-8", errors="replace") as f:
        for line in f:
            m = BENCH_LINE.search(line)
            if m:
                name, ns = m.group(1), float(m.group(2))
                by_name[name].append(ns)
    return dict(by_name)


def mean(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0.0


def friendly_label(key: str, category: str) -> str:
    """Short display label for a benchmark key."""
    if category == "assertion":
        # Assertion_GoSpecs_EqualTo -> GoSpecs EqualTo
        key = key.replace("Assertion_", "", 1)
        key = key.replace("_", " ")
        return key
    if category == "hooks":
        return key.replace("Hooks_", "", 1)
    if category == "runner":
        # Suite_100 -> 100 specs
        key = key.replace("Suite_", "", 1)
        return f"{key} specs"
    return key.replace("_", " ")


def category_from_key(key: str) -> str:
    if key.startswith("Assertion_"):
        return "assertion"
    if key.startswith("Hooks_"):
        return "hooks"
    if key.startswith("Suite_"):
        return "runner"
    return "other"


def plot_bar_chart(
    out_path: Path,
    title: str,
    items: list[tuple[str, float]],
) -> None:
    """Draw a bar chart: labels, ns/op, and relative to fastest (×)."""
    if not items:
        return
    labels = [friendly_label(name, category_from_key(name)) for name, _ in items]
    values = [v for _, v in items]
    fastest = min(values)
    ratios = [v / fastest if fastest else 1.0 for v in values]

    fig, ax = plt.subplots(figsize=(10, 5))
    x = range(len(labels))
    bars = ax.bar(x, values, color=plt.cm.viridis([r / max(ratios) for r in ratios]), edgecolor="gray", linewidth=0.5)
    ax.set_xticks(x)
    ax.set_xticklabels(labels, rotation=25, ha="right")
    ax.set_ylabel("ns/op")
    ax.set_title(title)
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)

    # Annotate bars with ns/op and × ratio
    for i, (bar, val, ratio) in enumerate(zip(bars, values, ratios)):
        ax.text(
            bar.get_x() + bar.get_width() / 2,
            bar.get_height() + (max(values) * 0.02),
            f"{val:.1f} ns  (×{ratio:.2f})",
            ha="center",
            va="bottom",
            fontsize=8,
            rotation=0,
        )
    plt.tight_layout()
    plt.savefig(out_path, dpi=120, bbox_inches="tight")
    plt.close()
    print(f"Wrote {out_path}")


def main() -> int:
    input_path = Path(sys.argv[1]) if len(sys.argv) > 1 else DEFAULT_INPUT
    if not input_path.is_file():
        print(f"Input file not found: {input_path}", file=sys.stderr)
        return 2
    results_dir = input_path.parent
    data = parse_benchmarks(input_path)
    if not data:
        print("No benchmark lines found in input.", file=sys.stderr)
        return 1

    # Average ns/op per benchmark name
    means = {k: mean(v) for k, v in data.items()}

    # Group by category
    assertion_keys = [k for k in means if k.startswith("Assertion_")]
    hooks_keys = [k for k in means if k.startswith("Hooks_")]
    runner_keys = [k for k in means if k.startswith("Suite_")]

    if assertion_keys:
        items = sorted(
            [(k, means[k]) for k in assertion_keys],
            key=lambda x: x[1],
        )
        plot_bar_chart(
            results_dir / "assertion_chart.png",
            "Assertion benchmarks (ns/op, × vs fastest)",
            items,
        )
    else:
        print("Warning: no Assertion_* benchmarks; skipping assertion_chart.png")

    if hooks_keys:
        items = sorted(
            [(k, means[k]) for k in hooks_keys],
            key=lambda x: x[1],
        )
        plot_bar_chart(
            results_dir / "hooks_chart.png",
            "Hooks benchmarks (ns/op, × vs fastest)",
            items,
        )
    else:
        print("Warning: no Hooks_* benchmarks; skipping hooks_chart.png")

    if runner_keys:
        # Sort by suite size (100, 1000, 10000, 50000)
        def suite_order(k: str) -> int:
            m = re.search(r"Suite_(\d+)", k)
            return int(m.group(1)) if m else 0

        items = sorted(
            [(k, means[k]) for k in runner_keys],
            key=lambda x: suite_order(x[0]),
        )
        plot_bar_chart(
            results_dir / "runner_chart.png",
            "Suite size benchmarks (ns/op, × vs fastest)",
            items,
        )
    else:
        print("Warning: no Suite_* benchmarks; skipping runner_chart.png")

    return 0


if __name__ == "__main__":
    sys.exit(main())
