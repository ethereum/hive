import $ from 'jquery';

const NEWLINE = 10;

// LineDecoder is a utility for incrementally decoding lines of text from an input stream.
class LineDecoder {
    constructor(length) {
        this._length = length;
        this._remaining = length;
    }

    _decoder = new TextDecoder('utf-8');
    _queue = [];    // input buffers
    _textbuf = '';  // unfinished line text
    _length = 0;    // output position, i.e. sum of bytes of all decoded text.
    _remaining = 0; // output position, i.e. sum of bytes of all decoded text.
    _offset = 0;    // offset into current buffer.
    _inp = 0;		// input position, i.e. sum of all buffers ever used.

    // inputPosition returns the amount of added input.
    get inputPosition() { return this._inp; }

    // outputLength returns the amount of produced output in bytes.
    get outputPosition() { return this._length - this._remaining; }

    // atEOF returns true when the end of input was reached.
    atEOF() { this._remaining <= 0; }

    // pushBytes adds input data to the decoder.
    pushBytes(bytes) {
        this._inp += bytes.length;
        this._queue.push(bytes);
        return this;
    }

    // decodeLine returns the next line as a string. When there isn't enough buffered
    // input data for a complete line, or all input was decoded, it returns false.
    decodeLine() {
        while (this._queue.length > 0 && !this.atEOF()) {
            var buf = this._queue[0];
            if (this._offset >= buf.length-1) {
                this._queue.shift();
                this._offset = 0;
                continue;
            }

            // Find next newline in buffer.
            let ofs = this._offset;
            let nextNL = buf.indexOf(NEWLINE, ofs);
            if (nextNL >= 0 && nextNL < this._remaining) {
                let len = nextNL - ofs;
                let slice = new Uint8Array(buf.buffer, ofs, len);
                let line = this._textbuf + this._decoder.decode(slice) + '\n';
                this._textbuf = '';
                this._remaining -= len + 1;
                this._offset += len + 1;
                return line;
            }
            // No newline in this buffer, consume it fully.
            let len = Math.min(buf.length-ofs, this._remaining);
            let slice = new Uint8Array(buf.buffer, ofs, len);
            this._textbuf += this._decoder.decode(slice, {stream: true});
            this._offset += len;
            this._remaining -= len;
            if (this.atEOF()) {
                return this._textbuf;
            }
        }
        return false;
    }
}

// ReverseBuffer decodes lines of text from input. This is intended for decoding the lines
// of a file in reverse, starting at the end. Unlike LineDecoder, it does not use a 'streaming'
// approach, i.e. all input data is kept in the buffer.
class ReverseBuffer {
    _data = new Uint8Array(0);
    _offset = 0;

    constructor(length) {
        this._length = length;
        this._outputPosition = 0;
    }

    get inputPosition() {
        return this._length - this._data.length;
    }

    get outputPosition() {
        return this._outputPosition;
    }

    // pushBytes delivers input into the buffer.
    pushBytes(bytes) {
        // var nl = bytes.indexOf(NEWLINE);
        // if (nl == -1) {
        //     if (this.firstNewline != -1) {
        //         // Shift the index of the first known newline position because the new
        //         // buffer will be prepended to the existing buffer.
        //         this.firstNewline += bytes.length;
        //     }
        // } else {
        //     this.lines++;
        //     this.firstNewline = nl;
        // }

        let newbuffer = new Uint8Array(this._data.length + bytes.length);
        newbuffer.set(bytes);
        newbuffer.set(this._data, bytes.length);
        this._data = newbuffer;
        this._offset += bytes.length;
        return this;
    }

    // decodeLine removes the one line from the buffer and returns it.
    decodeLine() {
        let pos = this._data.lastIndexOf(NEWLINE, this._offset);
        if (!pos) {
            return null;
        }
        this._offset = pos;
        this._outputPosition -= pos;
        
        let linedata = this._data.slice(pos, this._offset);
        let dec = new TextDecoder("utf-8");
        return dec.decode(linedata);
    }
}

// LogLoader provides incremental access to log files.
export class LogLoader {
    constructor(logFileName, offsets) {
        this.logFileName = logFileName;
        this.offsets = offsets;
        this.length = offsets.end - offsets.begin;
    }

    // headAndTailLines returns up to n lines from the beginning and
    // end of the log.
    async headAndTailLines(n) {
        var head = [];
        var headEndPosition = 0;
        var tail = [];

        // Read head lines first.
        let eofReached = await this._iterHeadLines(function (line, offset) {
            headEndPosition = offset;
            head.push(line);
            return head.length <= n;
        });
        if (eofReached || head.length >= n) {
            return {head, tail};
        }

        // Now read from tail. This stops when the read enters the
        // region already covered by the head.
        await this._iterTailLines(function (line, offset) {
            if (offset < headEndPosition) {
                return false;
            }
            tail.unshift(line);
            return tail.length <= n;
        });

        return {head, tail};
    }
    
    // headLines returns the first n lines of output.
    async headLines(n) {
        return await this._collectLines(n, this._iterHeadLines);
    }

    // tailLines returns the last n lines of output.
    async tailLines(n) {
        return await this._collectLines(n, this._iterTailLines);
    }

    async _collectLines(n, collector) {
        let lines = [];
        await collector(function (line) {
            lines.push(line);
            return lines.length < n;
        });
        return lines;
    }

    // iterLines reads text lines starting at the head of the file and calls fn
    // for each line. The second argument to fn is absolute read position at the start
    // of the line.
    // When the callback function returns false, iteration is aborted.
    async iterLines(func) {
        let dec = new LineDecoder(this.length);
        while (!dec.atEOF()) {
            let line = dec.decodeLine();
            if (!line) {
                // Decoder wants more input.
                let start = dec.inputPosition;
                let end = Math.max(start+1024, this.length);
                await this._fetchRange(start, end, dec);
                continue;
            }
            // A line was decoded.
            if (!func(line, dec.outputPosition)) {
                return;
            }
        }
    }

    async _iterTailLines(func) {
        let dec = new ReverseBuffer(this.length);
        while (dec.outputPosition > 0) {
            let line = dec.decodeLine();
            if (!line) {
                // Decoder wants more input.
                let end = dec.inputPosition;
                let start = Math.min(0, end-1024);
                await this._fetchRange(start, end, dec);
                continue;
            }
            // A line was decoded.
            if (!func(line, dec.outputPosition)) {
                return;
            }
        }
    }

    async _fetchRange(begin, end, buffer) {
        let rangeBegin = this.offsets.begin + begin;
        let rangeEnd = this.offsets.begin + end;
        let range = 'bytes=' + rangeBegin + '-' + rangeEnd;
        console.log('fetching range:', this.logFileName, 'range:', range);
        let options = {method: 'GET', headers: {'Range': range}};

        // TODO: handle range unavailable
        let response = await fetch(this.logFileName, options);
        let reader = response.body.getReader();
        for (;;) {
            let { done, value } = await reader.read();
            if (done) {
                break;
            }
            buffer.pushBytes(value);
        }
    }
}

// export function testlog() {
//     return new TestLog('/results/1674480438-d58c15919f43236e1e9186938a24b9ca.json-testlog.txt', {begin: 10042, end: 53219});
// }
// 
// export async function testit() {
//     let log = new tm.TestLog('/results/1674480438-d58c15919f43236e1e9186938a24b9ca.json-testlog.txt', {begin: 10042, end: 53219});
//     return await log.headLines(10);
// }
