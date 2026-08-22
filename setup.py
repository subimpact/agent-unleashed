from setuptools import setup, find_packages

setup(
    name="antigravity-unleashed",
    version="1.0.0",
    packages=find_packages(),
    install_requires=[
        "pyyaml>=6.0",
        "pydantic>=2.0.0",
        "aiohttp>=3.8.3",
        "click>=8.1.3",
    ],
    entry_points={
        "console_scripts": [
            "agy-ul=src.daemon:main",
            "antigravity-unleashed=src.daemon:main",
        ],
    },
)
