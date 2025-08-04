// splitHeadTail splits the given text, getting n lines from both the beginning and the
// end of the text.
export function splitHeadTail(text, maxLines) {
    let totalLines = countLines(text);

    var offset = 0, end = 0, lineNumber = 0;
    var head = [];
    var tail = [];
    var hiddenLines = 0;
    while (end < text.length) {
        // Find bounding indexes of the next line.
        end = text.indexOf('\n', offset);
        if (end == -1) {
            end = text.length;
        }
        let begin = offset;
        offset = end+1;

        // Collect lines in the visible range.
        let inPrefix = lineNumber < maxLines;
        let inSuffix = lineNumber > (totalLines-maxLines);
        if (inPrefix || inSuffix) {
            let line = text.substring(begin, end);
            if (lineNumber < totalLines-1) {
                line += '\n';
            }
            if (inPrefix) {
                head.push(line);
            } else {
                tail.push(line);
            }
        } else {
            hiddenLines++;
        }
        lineNumber++;
    }

    if (hiddenLines === 0) {
        head = head.concat(tail);
        tail = [];
    }
    return {head, tail, hiddenLines};
}

// countLines returns the number of lines in the given string.
function countLines(text) {
    var lines = 0, offset = 0;
    for (;;) {
        lines++;
        offset = text.indexOf('\n', offset);
        if (offset == -1) {
            return lines;
        }
        offset++;
    }
}

const NEWLINE = 10;
const LOADER_CHUNK_SIZE = 262144;

// Loader provides incremental access to log files.
export class Loader {
    constructor(logFileName, offsets) {
        // Ensure file offsets are valid.
        if (offsets.begin > offsets.end) {
            throw new Error(`invalid offsets: ${offsets.begin} > ${offsets.end}`);
        }
        this.logFileName = logFileName;
        this.length = offsets.end - offsets.begin;
        this.offsets = offsets;
    }

    // text fetches the entire log.
    async text(progressCallback) {
        let response = await this._fetchRange(0, this.length-1);
        let reader = response.body.getReader();
        if (progressCallback) {
            progressCallback(0, this.length);
        }

        let received = 0;
        let decoder = new TextDecoder();
        let text = '';
        for(;;) {
            const {done, value} = await reader.read();
            if (value) {
                received += value.length;
                text += decoder.decode(value, {stream: !done});
                if (progressCallback) {
                    progressCallback(received, this.length);
                }
            }
            if (done) {
                break;
            }
        }
        return text;
    }

    // headAndTailLines returns up to n lines from the beginning and
    // end of the log.
    async headAndTailLines(n, maxBytes) {
        var head = [];
        var headEndPosition = 0;
        var tail = [];

        // Read head lines first.
        let eofReached = await this.iterLines(function (line, offset) {
            headEndPosition = offset;
            head.push(line);
            let tooMuchData = maxBytes && offset > maxBytes && head.length > 0;
            return head.length < n && !tooMuchData;
        });
        if (eofReached || head.length < n) {
            return {head, tail};
        }

        // Now read from tail. This stops when the read enters the
        // region already covered by the head.
        let linkWithHead = false;
        const totalSize = this.length;
        await this._iterTailLines(function (line, offset) {
            if (offset < headEndPosition) {
                linkWithHead = true;
                return false;
            }
            tail.unshift(line);
            let tailSize = totalSize - offset;
            let tooMuchData = maxBytes && tailSize > maxBytes && tail.length > 0;
            return tail.length < n && !tooMuchData;
        });
        if (linkWithHead) {
            head = head.concat(tail);
            tail = [];
        }
        return {head, tail};
    }

    // iterLines reads text lines starting at the head of the file and calls fn
    // for each line. The second argument to fn is absolute read position at the end
    // of the line.
    //
    // When the callback function returns false, iteration is aborted.
    async iterLines(func) {
        let dec = new LineDecoder(this.length);
        while (!dec.atEOF()) {
            let text = dec.decode();
            if (!text) {
                // Decoder wants more input.
                let start = dec.inputPosition;
                let end = Math.min(start+LOADER_CHUNK_SIZE, this.length);
                await this._fetchRangeIntoBuffer(start, end, dec);
                continue;
            }
            // One or more lines were decoded.
            let outputPos = dec.outputPosition - text.length;
            let pos = 0;
            while (pos < text.length) {
                let nl = text.indexOf('\n', pos);
                let end = (nl < 0) ? text.length : nl+1;
                let line = text.substring(pos, end);
                pos = end;
                if (!func(line, outputPos + pos)) {
                    return;
                }
            }
        }
    }

    async _iterTailLines(func) {
        let dec = new ReverseBuffer(this.length);
        let first = true;
        while (dec.outputPosition > 0) {
            let line = dec.decode();
            if (!line) {
                // Decoder wants more input.
                let end = dec.inputPosition;
                let start = Math.max(0, end-LOADER_CHUNK_SIZE);
                await this._fetchRangeIntoBuffer(start, end, dec);
                continue;
            }
            if (first && line === '\n') {
                first = false;
                continue;
            }
            // A line was decoded.
            if (!func(line, dec.outputPosition)) {
                return;
            }
        }
    }

    async _fetchRangeIntoBuffer(begin, end, buffer) {
        let response = await this._fetchRange(begin, end);
        let blob = new Uint8Array(await response.arrayBuffer());
        buffer.pushBytes(blob);
    }

    async _fetchRange(begin, end) {
        let rangeBegin = this.offsets.begin + begin;
        let rangeEnd = this.offsets.begin + end;
        let range = 'bytes=' + rangeBegin + '-' + rangeEnd;
        console.log('fetching:', this.logFileName, 'range:', range);
        let options = {method: 'GET', headers: {'Range': range}};
        let response = await fetch(this.logFileName, options);
        if (!response.ok) {
            let status = `${response.status} ${response.statusText}`;
            throw new Error(`load ${this.logFileName} (range ${rangeBegin}-${rangeEnd}) failed: ${status}`);
        }
        return response;
    }
}

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
    atEOF() { return this._remaining <= 0; }

    // pushBytes adds input data to the decoder.
    pushBytes(bytes) {
        this._inp += bytes.length;
        this._queue.push(bytes);
        return this;
    }

    // decode returns one or more decoded lines as a string. When there isn't enough
    // buffered input data for a complete line, or all input was decoded, it returns
    // false.
    decode() {
        while (this._queue.length > 0 && !this.atEOF()) {
            var buf = this._queue[0];

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
            // Remove the buffer.
            this._offset = 0;
            this._queue.shift();

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
    _offset = 0; // index before the last found newline in _data

    constructor(length) {
        this._length = length;
        this._outputPosition = length;
    }

    get inputPosition() {
        return this._length - this._data.length - 1;
    }

    get outputPosition() {
        return this._outputPosition;
    }

    // pushBytes delivers input into the buffer.
    pushBytes(bytes) {
        let newbuffer = new Uint8Array(this._data.length + bytes.length);
        newbuffer.set(bytes);
        newbuffer.set(this._data, bytes.length);
        this._data = newbuffer;
        this._offset += bytes.length;
        return this;
    }

    // decode removes the one line from the buffer and returns it.
    decode() {
        if (this._offset <= 0) {
            return null;
        }
        let pos = this._data.lastIndexOf(NEWLINE, this._offset);
        if (pos == -1) {
            return null;
        }
        let lineLength = this._offset - pos;
        let lineBytes = this._data.slice(pos+1, this._offset+1);
        let line = new TextDecoder('utf-8').decode(lineBytes);
        this._offset = pos - 1;
        this._outputPosition -= lineLength;
        return line + '\n';
    }
}
