# modify Makefile

cd quickjs
open Makefile
find prefix=/usr/local
replace prefix=../

# build quickjs

```sh
make -j8 && make install
```
