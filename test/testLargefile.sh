#!/bin/bash

cat largeFileIn.txt | nc -w 1 localhost 20001 > out.tmp

md5 ./largeFileIn.txt ./out.tmp
