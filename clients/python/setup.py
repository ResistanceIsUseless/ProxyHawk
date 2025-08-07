#!/usr/bin/env python3
"""
Setup script for ProxyHawk Python Client Library.
"""

from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="proxyhawk-client",
    version="1.0.0",
    author="ProxyHawk Contributors",
    author_email="proxyhawk@example.com",
    description="Python client library for ProxyHawk geographic DNS testing service",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/ResistanceIsUseless/ProxyHawk",
    packages=find_packages(),
    classifiers=[
        "Development Status :: 4 - Beta",
        "Intended Audience :: Developers",
        "Topic :: Internet :: WWW/HTTP",
        "Topic :: Security",
        "Topic :: System :: Networking",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.7",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
    ],
    python_requires=">=3.7",
    install_requires=[
        "websockets>=10.0",
    ],
    extras_require={
        "dev": [
            "pytest>=6.0",
            "pytest-asyncio>=0.18.0",
            "black>=22.0",
            "flake8>=4.0",
        ],
    },
    entry_points={
        "console_scripts": [
            "proxyhawk-client=proxyhawk_client:main",
        ],
    },
)