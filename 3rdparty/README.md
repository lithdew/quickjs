# modify Makefile

1. cd quickjs
2. open Makefile
3. find prefix=/usr/local
4. replace prefix=../

# build quickjs

```sh
make -j8 && make install
```
