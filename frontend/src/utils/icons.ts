/*!
 * The outline icons of Gezgin's buttons (K106) are Tabler Icons 3.48.0, https://tabler.io/icons:
 *
 * MIT License
 *
 * Copyright (c) 2020-2026 Paweł Kuna
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE.
 */

// An outline icon: its paths on a 24 by 24 grid, stroked, or filled when it is the filled form.
export interface OutlineIcon {
  paths: string[];
  filled?: boolean;
}

// outlineIcons draws a button's icon, keyed by the Material Icons name the button is given; a
// name not here is still drawn by the Material Icons font.
export const outlineIcons: Record<string, OutlineIcon> = {
  // user-circle
  account_circle: {
    paths: [
      "M3 12a9 9 0 1 0 18 0a9 9 0 1 0 -18 0",
      "M9 10a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M6.168 18.849a4 4 0 0 1 3.832 -2.849h4a4 4 0 0 1 3.834 2.855",
    ],
  },
  // plus
  add: {
    paths: ["M12 5l0 14", "M5 12l14 0"],
  },
  // file-zip
  archive: {
    paths: [
      "M6 20.735a2 2 0 0 1 -1 -1.735v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2h-1",
      "M11 17a2 2 0 0 1 2 2v2a1 1 0 0 1 -1 1h-2a1 1 0 0 1 -1 -1v-2a2 2 0 0 1 2 -2",
      "M11 5l-1 0",
      "M13 7l-1 0",
      "M11 9l-1 0",
      "M13 11l-1 0",
      "M11 13l-1 0",
      "M13 15l-1 0",
    ],
  },
  // arrow-left
  arrow_back: {
    paths: ["M5 12l14 0", "M5 12l6 6", "M5 12l6 -6"],
  },
  // arrow-right
  arrow_forward: {
    paths: ["M5 12l14 0", "M13 18l6 -6", "M13 6l6 6"],
  },
  // arrow-up
  arrow_upward: {
    paths: ["M12 5l0 14", "M18 11l-6 -6", "M6 11l6 -6"],
  },
  // circle-check
  check_circle: {
    paths: ["M3 12a9 9 0 1 0 18 0a9 9 0 1 0 -18 0", "M9 12l2 2l4 -4"],
  },
  // chevron-right
  chevron_right: {
    paths: ["M9 6l6 6l-6 6"],
  },
  // x
  close: {
    paths: ["M18 6l-12 12", "M6 6l12 12"],
  },
  // copy
  content_copy: {
    paths: [
      "M7 9.667a2.667 2.667 0 0 1 2.667 -2.667h8.666a2.667 2.667 0 0 1 2.667 2.667v8.666a2.667 2.667 0 0 1 -2.667 2.667h-8.666a2.667 2.667 0 0 1 -2.667 -2.667l0 -8.666",
      "M4.012 16.737a2.005 2.005 0 0 1 -1.012 -1.737v-10c0 -1.1 .9 -2 2 -2h10c.75 0 1.158 .385 1.5 1",
    ],
  },
  // folder-plus
  create_new_folder: {
    paths: [
      "M12 19h-7a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v3.5",
      "M16 19h6",
      "M19 16v6",
    ],
  },
  // trash
  delete: {
    paths: [
      "M4 7l16 0",
      "M10 11l0 6",
      "M14 11l0 6",
      "M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12",
      "M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3",
    ],
  },
  // trash-x
  delete_forever: {
    paths: [
      "M4 7h16",
      "M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12",
      "M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3",
      "M10 12l4 4m0 -4l-4 4",
    ],
  },
  // server
  dns: {
    paths: [
      "M3 7a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v2a3 3 0 0 1 -3 3h-12a3 3 0 0 1 -3 -3v-2",
      "M3 15a3 3 0 0 1 3 -3h12a3 3 0 0 1 3 3v2a3 3 0 0 1 -3 3h-12a3 3 0 0 1 -3 -3l0 -2",
      "M7 8l0 .01",
      "M7 16l0 .01",
    ],
  },
  // folder-share
  drive_file_move: {
    paths: [
      "M13 19h-8a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v4",
      "M16 22l5 -5",
      "M21 21.5v-4.5h-4.5",
    ],
  },
  // edit
  edit_note: {
    paths: [
      "M7 7h-1a2 2 0 0 0 -2 2v9a2 2 0 0 0 2 2h9a2 2 0 0 0 2 -2v-1",
      "M20.385 6.585a2.1 2.1 0 0 0 -2.97 -2.97l-8.415 8.385v3h3l8.385 -8.415",
      "M16 5l3 3",
    ],
  },
  // download
  file_download: {
    paths: [
      "M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2",
      "M7 11l5 5l5 -5",
      "M12 4l0 12",
    ],
  },
  // upload
  file_upload: {
    paths: [
      "M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2 -2v-2",
      "M7 9l5 -5l5 5",
      "M12 4l0 12",
    ],
  },
  // folder-open
  folder_open: {
    paths: [
      "M5 19l2.757 -7.351a1 1 0 0 1 .936 -.649h12.307a1 1 0 0 1 .986 1.164l-.996 5.211a2 2 0 0 1 -1.964 1.625h-14.026a2 2 0 0 1 -2 -2v-11a2 2 0 0 1 2 -2h4l3 3h7a2 2 0 0 1 2 2v2",
    ],
  },
  // layout-cards
  grid_view: {
    paths: [
      "M4 6a2 2 0 0 1 2 -2h2a2 2 0 0 1 2 2v12a2 2 0 0 1 -2 2h-2a2 2 0 0 1 -2 -2l0 -12",
      "M14 6a2 2 0 0 1 2 -2h2a2 2 0 0 1 2 2v6a2 2 0 0 1 -2 2h-2a2 2 0 0 1 -2 -2l0 -6",
    ],
  },
  // users
  group: {
    paths: [
      "M5 7a4 4 0 1 0 8 0a4 4 0 1 0 -8 0",
      "M3 21v-2a4 4 0 0 1 4 -4h4a4 4 0 0 1 4 4v2",
      "M16 3.13a4 4 0 0 1 0 7.75",
      "M21 21v-2a4 4 0 0 0 -3 -3.85",
    ],
  },
  // badge-hd
  hd: {
    paths: [
      "M3 7a2 2 0 0 1 2 -2h14a2 2 0 0 1 2 2v10a2 2 0 0 1 -2 2h-14a2 2 0 0 1 -2 -2v-10",
      "M14 9v6h1a2 2 0 0 0 2 -2v-2a2 2 0 0 0 -2 -2h-1",
      "M7 15v-6",
      "M10 15v-6",
      "M7 12h3",
    ],
  },
  // home
  home: {
    paths: [
      "M5 12l-2 0l9 -9l9 9l-2 0",
      "M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-7",
      "M9 21v-6a2 2 0 0 1 2 -2h2a2 2 0 0 1 2 2v6",
    ],
  },
  // info-circle
  info: {
    paths: [
      "M3 12a9 9 0 1 0 18 0a9 9 0 0 0 -18 0",
      "M12 9h.01",
      "M11 12h1v4h1",
    ],
  },
  // file
  insert_drive_file: {
    paths: [
      "M14 3v4a1 1 0 0 0 1 1h4",
      "M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2",
    ],
  },
  // link
  link: {
    paths: [
      "M9 15l6 -6",
      "M11 6l.463 -.536a5 5 0 0 1 7.071 7.072l-.534 .464",
      "M13 18l-.397 .534a5.068 5.068 0 0 1 -7.127 0a4.972 4.972 0 0 1 0 -7.071l.524 -.463",
    ],
  },
  // lock
  lock: {
    paths: [
      "M5 13a2 2 0 0 1 2 -2h10a2 2 0 0 1 2 2v6a2 2 0 0 1 -2 2h-10a2 2 0 0 1 -2 -2v-6",
      "M11 16a1 1 0 1 0 2 0a1 1 0 0 0 -2 0",
      "M8 11v-4a4 4 0 1 1 8 0v4",
    ],
  },
  // logout
  logout: {
    paths: [
      "M14 8v-2a2 2 0 0 0 -2 -2h-7a2 2 0 0 0 -2 2v12a2 2 0 0 0 2 2h7a2 2 0 0 0 2 -2v-2",
      "M9 12h12l-3 -3",
      "M18 15l3 -3",
    ],
  },
  // menu-2
  menu: {
    paths: ["M4 6l16 0", "M4 12l16 0", "M4 18l16 0"],
  },
  // forms
  mode_edit: {
    paths: [
      "M12 3a3 3 0 0 0 -3 3v12a3 3 0 0 0 3 3",
      "M6 3a3 3 0 0 1 3 3v12a3 3 0 0 1 -3 3",
      "M13 7h7a1 1 0 0 1 1 1v8a1 1 0 0 1 -1 1h-7",
      "M5 7h-1a1 1 0 0 0 -1 1v8a1 1 0 0 0 1 1h1",
      "M17 12h.01",
      "M13 12h.01",
    ],
  },
  // dots-vertical
  more_vert: {
    paths: [
      "M11 12a1 1 0 1 0 2 0a1 1 0 1 0 -2 0",
      "M11 19a1 1 0 1 0 2 0a1 1 0 1 0 -2 0",
      "M11 5a1 1 0 1 0 2 0a1 1 0 1 0 -2 0",
    ],
  },
  // file-plus
  note_add: {
    paths: [
      "M14 3v4a1 1 0 0 0 1 1h4",
      "M17 21h-10a2 2 0 0 1 -2 -2v-14a2 2 0 0 1 2 -2h7l5 5v11a2 2 0 0 1 -2 2",
      "M12 11l0 6",
      "M9 14l6 0",
    ],
  },
  // external-link
  open_in_new: {
    paths: [
      "M12 6h-6a2 2 0 0 0 -2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2 -2v-6",
      "M11 13l9 -9",
      "M15 4h5v5",
    ],
  },
  // user
  person: {
    paths: [
      "M8 7a4 4 0 1 0 8 0a4 4 0 0 0 -8 0",
      "M6 21v-2a4 4 0 0 1 4 -4h4a4 4 0 0 1 4 4v2",
    ],
  },
  // user-plus
  person_add: {
    paths: [
      "M8 7a4 4 0 1 0 8 0a4 4 0 0 0 -8 0",
      "M16 19h6",
      "M19 16v6",
      "M6 21v-2a4 4 0 0 1 4 -4h4",
    ],
  },
  // eye
  preview: {
    paths: [
      "M10 12a2 2 0 1 0 4 0a2 2 0 0 0 -4 0",
      "M21 12c-2.4 4 -5.4 6 -9 6c-3.6 0 -6.6 -2 -9 -6c2.4 -4 5.4 -6 9 -6c3.6 0 6.6 2 9 6",
    ],
  },
  // refresh
  refresh: {
    paths: [
      "M20 11a8.1 8.1 0 0 0 -15.5 -2m-.5 -4v4h4",
      "M4 13a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4",
    ],
  },
  // minus
  remove: {
    paths: ["M5 12l14 0"],
  },
  // restore
  restore_from_trash: {
    paths: [
      "M3.06 13a9 9 0 1 0 .49 -4.087",
      "M3 4.001v5h5",
      "M11 12a1 1 0 1 0 2 0a1 1 0 1 0 -2 0",
    ],
  },
  // device-floppy
  save: {
    paths: [
      "M6 4h10l4 4v10a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2v-12a2 2 0 0 1 2 -2",
      "M10 14a2 2 0 1 0 4 0a2 2 0 1 0 -4 0",
      "M14 4l0 4l-6 0l0 -4",
    ],
  },
  // search
  search: {
    paths: ["M3 10a7 7 0 1 0 14 0a7 7 0 1 0 -14 0", "M21 21l-6 -6"],
  },
  // settings
  settings: {
    paths: [
      "M10.325 4.317c.426 -1.756 2.924 -1.756 3.35 0a1.724 1.724 0 0 0 2.573 1.066c1.543 -.94 3.31 .826 2.37 2.37a1.724 1.724 0 0 0 1.065 2.572c1.756 .426 1.756 2.924 0 3.35a1.724 1.724 0 0 0 -1.066 2.573c.94 1.543 -.826 3.31 -2.37 2.37a1.724 1.724 0 0 0 -2.572 1.065c-.426 1.756 -2.924 1.756 -3.35 0a1.724 1.724 0 0 0 -2.573 -1.066c-1.543 .94 -3.31 -.826 -2.37 -2.37a1.724 1.724 0 0 0 -1.065 -2.572c-1.756 -.426 -1.756 -2.924 0 -3.35a1.724 1.724 0 0 0 1.066 -2.573c-.94 -1.543 .826 -3.31 2.37 -2.37c1 .608 2.296 .07 2.572 -1.065",
      "M9 12a3 3 0 1 0 6 0a3 3 0 0 0 -6 0",
    ],
  },
  // share
  share: {
    paths: [
      "M3 12a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M15 6a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M15 18a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M8.7 10.7l6.6 -3.4",
      "M8.7 13.3l6.6 3.4",
    ],
  },
  // share-off
  share_off: {
    paths: [
      "M3 12a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M15 6a3 3 0 1 0 6 0a3 3 0 1 0 -6 0",
      "M15.861 15.896a3 3 0 0 0 4.265 4.22m.578 -3.417a3.012 3.012 0 0 0 -1.507 -1.45",
      "M8.7 10.7l1.336 -.688m2.624 -1.352l2.64 -1.36",
      "M8.7 13.3l6.6 3.4",
      "M3 3l18 18",
    ],
  },
  // star (filled)
  star: {
    paths: [
      "M8.243 7.34l-6.38 .925l-.113 .023a1 1 0 0 0 -.44 1.684l4.622 4.499l-1.09 6.355l-.013 .11a1 1 0 0 0 1.464 .944l5.706 -3l5.693 3l.1 .046a1 1 0 0 0 1.352 -1.1l-1.091 -6.355l4.624 -4.5l.078 -.085a1 1 0 0 0 -.633 -1.62l-6.38 -.926l-2.852 -5.78a1 1 0 0 0 -1.794 0l-2.853 5.78z",
    ],
    filled: true,
  },
  // star
  star_border: {
    paths: [
      "M12 17.75l-6.172 3.245l1.179 -6.873l-5 -4.867l6.9 -1l3.086 -6.253l3.086 6.253l6.9 1l-5 4.867l1.179 6.873l-6.158 -3.245",
    ],
  },
  // adjustments
  tune: {
    paths: [
      "M4 10a2 2 0 1 0 4 0a2 2 0 0 0 -4 0",
      "M6 4v4",
      "M6 12v8",
      "M10 16a2 2 0 1 0 4 0a2 2 0 0 0 -4 0",
      "M12 4v10",
      "M12 18v2",
      "M16 7a2 2 0 1 0 4 0a2 2 0 0 0 -4 0",
      "M18 4v1",
      "M18 9v11",
    ],
  },
  // package-export
  unarchive: {
    paths: [
      "M12 21l-8 -4.5v-9l8 -4.5l8 4.5v4.5",
      "M12 12l8 -4.5",
      "M12 12v9",
      "M12 12l-8 -4.5",
      "M15 18h7",
      "M19 15l3 3l-3 3",
    ],
  },
  // layout-list
  view_list: {
    paths: [
      "M4 6a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v2a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2l0 -2",
      "M4 16a2 2 0 0 1 2 -2h12a2 2 0 0 1 2 2v2a2 2 0 0 1 -2 2h-12a2 2 0 0 1 -2 -2l0 -2",
    ],
  },
  // layout-grid
  view_module: {
    paths: [
      "M4 5a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1l0 -4",
      "M14 5a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1l0 -4",
      "M4 15a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1l0 -4",
      "M14 15a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v4a1 1 0 0 1 -1 1h-4a1 1 0 0 1 -1 -1l0 -4",
    ],
  },
  // eye-off
  visibility_off: {
    paths: [
      "M10.585 10.587a2 2 0 0 0 2.829 2.828",
      "M16.681 16.673a8.717 8.717 0 0 1 -4.681 1.327c-3.6 0 -6.6 -2 -9 -6c1.272 -2.12 2.712 -3.678 4.32 -4.674m2.86 -1.146a9.055 9.055 0 0 1 1.82 -.18c3.6 0 6.6 2 9 6c-.666 1.11 -1.379 2.067 -2.138 2.87",
      "M3 3l18 18",
    ],
  },
};
