/** The last element of a path, which is what a person calls the folder.
 * @param {string} path */
export const folderName = (path) => path.split(/[\\/]/).filter(Boolean).pop() ?? path;
