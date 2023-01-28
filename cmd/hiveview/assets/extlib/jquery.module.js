// This is a module wrapper for jQuery.
// In app files, use
//
//     import { $ } from '../extlib/jquery.module.js'
//
// to load jQuery.

import './jquery-3.6.3.min.js';
export const $ = window.$;
export const jQuery = window.jQuery;
