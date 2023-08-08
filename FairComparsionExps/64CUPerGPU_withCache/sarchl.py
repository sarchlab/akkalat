"""SARCHL

This script provides the commonly use utilities for the Scalable Architecture Lab. 
"""

primary_color = "#3db2d1"
secondary_color = "#f1843f"

primary_shades = [
    "#02090a",
    "#071a1f",
    "#0c2b34",
    "#113d48",
    "#164e5d",
    "#1b5f71",
    "#207186",
    "#25829b",
    "#2a93af",
    "#2ea5c4",
    "#3bb1d1",
    "#50b9d5",
    "#64c2da",
    "#79cadf",
    "#8ed2e4",
    "#a2dae9",
    "#b7e2ee",
    "#cbebf3",
    "#e0f3f8",
    "#f5fbfd",
]


def sequential_palette(num_shades):
    """Generate a sequential palette based on the primary shades

    Args:
        num_shades (int): Number of shades to generate

    Returns:
        list: List of colors
    """

    if num_shades > len(primary_shades):
        raise ValueError("No more than 20 shades can be generated")

    interval = int(len(primary_shades) / num_shades)
    color_range = interval * num_shades
    offset = int(len(primary_shades) - color_range) // 2

    return primary_shades[offset::interval]
