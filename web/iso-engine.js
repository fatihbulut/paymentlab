/**
 * ISO 8583 Encoder/Decoder Engine
 * Single Source of Truth: spec.json
 */

class ISO8583Engine {
    constructor() {
        this.spec = null;
        this.ready = false;
    }

    async init() {
        try {
            const response = await fetch('/spec.json');
            this.spec = await response.json();
            this.ready = true;
            console.log('ISO8583 Engine initialized with spec version:', this.spec.version);
        } catch (error) {
            console.error('Failed to load ISO spec:', error);
            throw error;
        }
    }

    /**
     * Encode ISO message to hex
     */
    encode(message) {
        if (!this.ready) throw new Error('Engine not initialized');

        const fields = {};
        const bitmap = new Set();

        // Process each field in the message
        for (const [fieldNum, value] of Object.entries(message)) {
            if (fieldNum === 'MTI' || !value) continue;
            
            const fieldNumInt = parseInt(fieldNum);
            const fieldSpec = this.spec.fields[fieldNumInt];
            
            if (!fieldSpec) {
                console.warn(`Field ${fieldNumInt} not in spec, skipping`);
                continue;
            }

            // Format the field value according to spec
            const formattedValue = this._formatField(value, fieldSpec);
            fields[fieldNumInt] = formattedValue;
            bitmap.add(fieldNumInt);
        }

        // Build the message
        const mti = message.MTI || '0100';
        const bitmapHex = this._buildBitmap(bitmap);
        
        let messageHex = this._stringToHex(mti);
        messageHex += bitmapHex;

        // Add fields in order
        const sortedFields = Object.keys(fields).map(Number).sort((a, b) => a - b);
        for (const fieldNum of sortedFields) {
            messageHex += fields[fieldNum];
        }

        return messageHex;
    }

    /**
     * Decode hex to ISO message
     */
    decode(hexString) {
        if (!this.ready) throw new Error('Engine not initialized');
        
        let pos = 0;
        const message = {};

        // Parse MTI (4 chars = 8 hex digits)
        message.MTI = this._hexToString(hexString.substring(pos, pos + 8));
        pos += 8;

        // Parse bitmap (16 hex digits = 64 bits)
        const bitmapHex = hexString.substring(pos, pos + 16);
        pos += 16;
        const presentFields = this._parseBitmap(bitmapHex);

        // Parse each present field
        for (const fieldNum of presentFields) {
            const fieldSpec = this.spec.fields[fieldNum];
            if (!fieldSpec) {
                console.warn(`Field ${fieldNum} not in spec, stopping parse`);
                break;
            }

            const result = this._parseField(hexString, pos, fieldSpec);
            message[fieldSpec.name] = result.value;
            pos = result.newPos;
        }

        return message;
    }

    /**
     * Format field value according to spec
     */
    _formatField(value, spec) {
        let formatted = String(value);

        // Apply padding
        if (spec.format === 'fixed') {
            if (spec.padding === 'left') {
                formatted = formatted.padStart(spec.length, spec.padChar || '0');
            } else if (spec.padding === 'right') {
                formatted = formatted.padEnd(spec.length, spec.padChar || ' ');
            }
            // Truncate if too long
            if (formatted.length > spec.length) {
                formatted = formatted.substring(0, spec.length);
            }
        }

        // Add length prefix for variable fields
        let result = '';
        if (spec.format === 'llvar') {
            const len = formatted.length.toString().padStart(2, '0');
            result = this._stringToHex(len);
        } else if (spec.format === 'lllvar') {
            const len = formatted.length.toString().padStart(3, '0');
            result = this._stringToHex(len);
        }

        result += this._stringToHex(formatted);
        return result;
    }

    /**
     * Parse field from hex string
     */
    _parseField(hexString, pos, spec) {
        let length = spec.length;
        let dataStart = pos;

        // Handle variable length fields
        if (spec.format === 'llvar') {
            const lenHex = hexString.substring(pos, pos + 4); // 2 chars = 4 hex
            length = parseInt(this._hexToString(lenHex));
            dataStart = pos + 4;
        } else if (spec.format === 'lllvar') {
            const lenHex = hexString.substring(pos, pos + 6); // 3 chars = 6 hex
            length = parseInt(this._hexToString(lenHex));
            dataStart = pos + 6;
        }

        const dataHex = hexString.substring(dataStart, dataStart + (length * 2));
        const value = this._hexToString(dataHex);

        return {
            value: value,
            newPos: dataStart + (length * 2)
        };
    }

    /**
     * Build bitmap from field numbers
     */
    _buildBitmap(fieldNumbers) {
        const bitmap = new Array(64).fill(0);
        
        for (const fieldNum of fieldNumbers) {
            if (fieldNum >= 2 && fieldNum <= 64) {
                bitmap[fieldNum - 1] = 1;
            }
        }

        // Convert to hex (8 bytes = 16 hex chars)
        let hex = '';
        for (let i = 0; i < 64; i += 4) {
            const nibble = bitmap.slice(i, i + 4).join('');
            hex += parseInt(nibble, 2).toString(16).toUpperCase();
        }

        return hex;
    }

    /**
     * Parse bitmap to get present field numbers
     */
    _parseBitmap(bitmapHex) {
        const fields = [];
        
        for (let i = 0; i < bitmapHex.length; i++) {
            const nibble = parseInt(bitmapHex[i], 16);
            for (let bit = 3; bit >= 0; bit--) {
                const fieldNum = (i * 4) + (3 - bit) + 1;
                if (fieldNum <= 64 && (nibble & (1 << bit))) {
                    fields.push(fieldNum);
                }
            }
        }

        return fields.sort((a, b) => a - b);
    }

    /**
     * Extract and parse bitmap from hex message
     */
    extractBitmap(hexString) {
        if (!this.ready) throw new Error('Engine not initialized');
        
        // Skip MTI (4 chars = 8 hex digits)
        const mtiHex = hexString.substring(0, 8);
        
        // Check if secondary bitmap is present (bit 1 of primary bitmap)
        const primaryBitmapHex = hexString.substring(8, 24);
        const firstNibble = parseInt(primaryBitmapHex[0], 16);
        const hasSecondary = (firstNibble & 8) !== 0; // Bit 1 (MSB of first nibble)
        
        let secondaryBitmapHex = '';
        let totalBitmapHex = primaryBitmapHex;
        
        if (hasSecondary) {
            secondaryBitmapHex = hexString.substring(24, 40);
            totalBitmapHex = primaryBitmapHex + secondaryBitmapHex;
        }
        
        const presentFields = this._parseBitmap(totalBitmapHex);
        
        return {
            mti: this._hexToString(mtiHex),
            primaryBitmap: primaryBitmapHex,
            secondaryBitmap: secondaryBitmapHex,
            hasSecondary: hasSecondary,
            presentFields: presentFields,
            fullBitmap: totalBitmapHex
        };
    }

    /**
     * Get field numbers from message object
     */
    getFieldNumbers(message) {
        const fields = [];
        for (const [key, value] of Object.entries(message)) {
            if (key === 'MTI') continue;
            
            // Check if key is a field number
            const fieldNum = parseInt(key);
            if (!isNaN(fieldNum) && fieldNum >= 2 && fieldNum <= 192) {
                fields.push(fieldNum);
            } else {
                // Check if key is a field name
                for (const [num, spec] of Object.entries(this.spec.fields)) {
                    if (spec.name === key && value) {
                        fields.push(parseInt(num));
                        break;
                    }
                }
            }
        }
        return fields.sort((a, b) => a - b);
    }

    /**
     * Convert hex to ASCII readable format (MTI + Bitmap + Data)
     */
    hexToAscii(hexString) {
        if (!this.ready) throw new Error('Engine not initialized');
        
        let pos = 0;
        let result = '';

        // Parse MTI (4 chars = 8 hex digits)
        const mtiHex = hexString.substring(pos, pos + 8);
        const mti = this._hexToString(mtiHex);
        result += mti;
        pos += 8;

        // Parse bitmap (16 hex digits = 64 bits)
        const bitmapHex = hexString.substring(pos, pos + 16);
        result += bitmapHex;
        pos += 16;

        // Check if secondary bitmap is present
        const firstNibble = parseInt(bitmapHex[0], 16);
        const hasSecondary = (firstNibble & 8) !== 0;
        
        if (hasSecondary) {
            const secondaryBitmapHex = hexString.substring(pos, pos + 16);
            result += secondaryBitmapHex;
            pos += 16;
        }

        // Parse remaining data as ASCII
        const dataHex = hexString.substring(pos);
        if (dataHex) {
            const dataAscii = this._hexToString(dataHex);
            result += dataAscii;
        }

        return result;
    }

    /**
     * Get field spec by number
     */
    getFieldSpec(fieldNum) {
        return this.spec?.fields[fieldNum];
    }

    /**
     * Convert string to hex
     */
    _stringToHex(str) {
        let hex = '';
        for (let i = 0; i < str.length; i++) {
            hex += str.charCodeAt(i).toString(16).padStart(2, '0');
        }
        return hex.toUpperCase();
    }

    /**
     * Convert hex to string
     */
    _hexToString(hex) {
        let str = '';
        for (let i = 0; i < hex.length; i += 2) {
            str += String.fromCharCode(parseInt(hex.substring(i, i + 2), 16));
        }
        return str;
    }

    /**
     * Get field spec by number
     */
    getFieldSpec(fieldNum) {
        return this.spec?.fields[fieldNum];
    }

    /**
     * Get response code description
     */
    getResponseCodeDesc(code) {
        return this.spec?.responseCodes[code] || 'Unknown';
    }

    /**
     * Convert message object to field map (for display)
     */
    messageToFields(message) {
        const fields = [];
        
        for (const [key, value] of Object.entries(message)) {
            if (key === 'MTI') {
                fields.push({
                    number: 0,
                    name: 'MTI',
                    description: 'Message Type Indicator',
                    value: value
                });
                continue;
            }

            // Check if key is a field number or field name
            const fieldNum = parseInt(key);
            if (!isNaN(fieldNum)) {
                // Key is a field number
                const spec = this.spec.fields[key];
                if (spec && value) {
                    fields.push({
                        number: String(fieldNum),
                        name: spec.name,
                        description: spec.description,
                        value: value
                    });
                }
            } else {
                // Key is a field name - find field number
                for (const [num, spec] of Object.entries(this.spec.fields)) {
                    if (spec.name === key && value) {
                        fields.push({
                            number: String(num),
                            name: spec.name,
                            description: spec.description,
                            value: value
                        });
                        break;
                    }
                }
            }
        }

        return fields.sort((a, b) => a.number - b.number);
    }
}

// Global instance
const isoEngine = new ISO8583Engine();
