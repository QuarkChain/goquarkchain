TARGETS = native.go native_wrap.cxx

.PHONY: all

all: $(TARGETS)

$(TARGETS): qkchash.h native.i
	swig -go -cgo -c++ -intgosize 64 native.i


clean:
	rm -f $(TARGETS)