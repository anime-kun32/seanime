/**
 * OS module provides a platform-independent interface to operating system functionality.
 * This is a restricted subset of Go's os package with permission checks.
 */
declare namespace $os {
    /** The operating system (e.g., "darwin", "linux", "windows") */
    const platform: string

    /** The system architecture (e.g., "amd64", "arm64") */
    const arch: string

    /**
     * Creates and executes a new command with the given arguments.
     * Command execution is subject to permission checks.
     * @param name The name of the command to run
     * @param args The arguments to pass to the command
     * @returns A command object or an error if the command is not authorized
     */
    function cmd(name: string, ...args: string[]): Cmd;

    /**
     * Reads the entire file specified by path.
     * @param path The path to the file to read
     * @returns The file contents as a byte array
     * @throws Error if the path is not authorized for reading
     */
    function readFile(path: string): Uint8Array;

    /**
     * Writes data to the named file, creating it if necessary.
     * If the file exists, it is truncated.
     * @param path The path to the file to write
     * @param data The data to write to the file
     * @param perm The file mode (permissions)
     * @throws Error if the path is not authorized for writing
     */
    function writeFile(path: string, data: Uint8Array, perm: number): void;

    /**
     * Reads a directory, returning a list of directory entries.
     * @param path The path to the directory to read
     * @returns An array of directory entries
     * @throws Error if the path is not authorized for reading
     */
    function readDir(path: string): DirEntry[];

    /**
     * Returns the default directory to use for temporary files.
     * @returns The temporary directory path or empty string if not authorized
     */
    function tempDir(): string;

    /**
     * Changes the size of the named file.
     * @param path The path to the file to truncate
     * @param size The new size of the file
     * @throws Error if the path is not authorized for writing
     */
    function truncate(path: string, size: number): void;

    /**
     * Creates a new directory with the specified name and permission bits.
     * @param path The path of the directory to create
     * @param perm The permission bits
     * @throws Error if the path is not authorized for writing
     */
    function mkdir(path: string, perm: number): void;

    /**
     * Creates a directory named path, along with any necessary parents.
     * @param path The path of the directory to create
     * @param perm The permission bits
     * @throws Error if the path is not authorized for writing
     */
    function mkdirAll(path: string, perm: number): void;

    /**
     * Renames (moves) oldpath to newpath.
     * @param oldpath The source path
     * @param newpath The destination path
     * @throws Error if either path is not authorized for writing
     */
    function rename(oldpath: string, newpath: string): void;

    /**
     * Removes the named file or (empty) directory.
     * @param path The path to remove
     * @throws Error if the path is not authorized for writing
     */
    function remove(path: string): void;

    /**
     * Removes path and any children it contains.
     * @param path The path to remove recursively
     * @throws Error if the path is not authorized for writing
     */
    function removeAll(path: string): void;

    /**
     * Returns a FileInfo describing the named file.
     * @param path The path to get information about
     * @returns Information about the file
     * @throws Error if the path is not authorized for reading
     */
    function stat(path: string): FileInfo;


    /**
     * Cmd represents an external command being prepared or run.
     * A Cmd cannot be reused after calling its Run, Output or CombinedOutput methods.
     */
    interface Cmd {
        /**
         * Args holds command line arguments, including the command as Args[0].
         * If the Args field is empty or nil, Run uses {Path}.
         * In typical use, both Path and Args are set by calling Command.
         */
        args: string[];

        /**
         * If Cancel is non-nil, the command must have been created with CommandContext
         * and Cancel will be called when the command's Context is done.
         */
        cancel: (() => void);

        /**
         * Dir specifies the working directory of the command.
         * If Dir is the empty string, Run runs the command in the calling process's current directory.
         */
        dir: string;

        /**
         * Env specifies the environment of the process.
         * Each entry is of the form "key=value".
         * If Env is nil, the new process uses the current process's environment.
         */
        env: string[];

        /** Error information if the command failed */
        err: Error;

        /**
         * ExtraFiles specifies additional open files to be inherited by the new process.
         * It does not include standard input, standard output, or standard error.
         */
        extraFiles: any[];

        /**
         * Path is the path of the command to run.
         * This is the only field that must be set to a non-zero value.
         */
        path: string;

        /** Process is the underlying process, once started. */
        process?: any;

        /** ProcessState contains information about an exited process. */
        processState?: any;

        /** Standard error of the command */
        stderr: any;

        /** Standard input of the command */
        stdin: any;

        /** Standard output of the command */
        stdout: any;

        /** SysProcAttr holds optional, operating system-specific attributes. */
        sysProcAttr?: any;

        /**
         * If WaitDelay is non-zero, it bounds the time spent waiting on two sources of
         * unexpected delay in Wait: a child process that fails to exit after the associated
         * Context is canceled, and a child process that exits but leaves its I/O pipes unclosed.
         */
        waitDelay: number;

        /**
         * CombinedOutput runs the command and returns its combined standard output and standard error.
         * @returns The combined output as a string or byte array
         */
        combinedOutput(): string | number[];

        /**
         * Environ returns a copy of the environment in which the command would be run as it is currently configured.
         * @returns The environment variables as an array of strings
         */
        environ(): string[];

        /**
         * Output runs the command and returns its standard output.
         * @returns The standard output as a string or byte array
         */
        output(): string | number[];

        /**
         * Run starts the specified command and waits for it to complete.
         * The returned error is nil if the command runs, has no problems copying stdin, stdout,
         * and stderr, and exits with a zero exit status.
         */
        run(): void;

        /**
         * Start starts the specified command but does not wait for it to complete.
         * If Start returns successfully, the c.Process field will be set.
         */
        start(): void;

        /**
         * StderrPipe returns a pipe that will be connected to the command's standard error when the command starts.
         * @returns A readable stream for the command's standard error
         */
        stderrPipe(): any;

        /**
         * StdinPipe returns a pipe that will be connected to the command's standard input when the command starts.
         * @returns A writable stream for the command's standard input
         */
        stdinPipe(): any;

        /**
         * StdoutPipe returns a pipe that will be connected to the command's standard output when the command starts.
         * @returns A readable stream for the command's standard output
         */
        stdoutPipe(): any;

        /**
         * String returns a human-readable description of the command.
         * It is intended only for debugging.
         * @returns A string representation of the command
         */
        string(): string;

        /**
         * Wait waits for the command to exit and waits for any copying to stdin or copying from stdout or stderr to complete.
         * The command must have been started by Start.
         */
        wait(): void;
    }

    /**
     * FileInfo describes a file and is returned by stat.
     */
    interface FileInfo {
        /** Base name of the file */
        name(): string;

        /** Length in bytes for regular files; system-dependent for others */
        size(): number;

        /** File mode bits */
        mode(): FileMode;

        /** Modification time */
        modTime(): Date;

        /** Abbreviation for mode().isDir() */
        isDir(): boolean;

        /** Underlying data source (can return null) */
        sys(): any;
    }

    /**
     * DirEntry is an entry read from a directory.
     */
    interface DirEntry {
        /** Returns the name of the file (or subdirectory) described by the entry */
        name(): string;

        /** Reports whether the entry describes a directory */
        isDir(): boolean;

        /** Returns the type bits for the entry */
        type(): FileMode;

        /** Returns the FileInfo for the file or subdirectory described by the entry */
        info(): FileInfo;
    }

    /**
     * FileMode represents a file's mode and permission bits.
     */
    type FileMode = number;

    /**
     * Constants for file mode bits
     */
    declare namespace FileMode {
        /** Is a directory */
        const ModeDir: number

        /** Append-only */
        const ModeAppend: number

        /** Exclusive use */
        const ModeExclusive: number

        /** Temporary file */
        const ModeTemporary: number

        /** Symbolic link */
        const ModeSymlink: number

        /** Device file */
        const ModeDevice: number

        /** Named pipe (FIFO) */
        const ModeNamedPipe: number

        /** Unix domain socket */
        const ModeSocket: number

        /** Setuid */
        const ModeSetuid: number

        /** Setgid */
        const ModeSetgid: number

        /** Unix character device, when ModeDevice is set */
        const ModeCharDevice: number

        /** Sticky */
        const ModeSticky: number

        /** Non-regular file */
        const ModeIrregular: number

        /** Mask for the type bits. For regular files, none will be set */
        const ModeType: number

        /** Unix permission bits, 0o777 */
        const ModePerm: number
    }
}

/**
 * Filepath module provides functions to manipulate file paths in a way compatible with the target operating system.
 */
declare namespace $filepath {
    /**
     * Returns the last element of path.
     * @param path The path to get the base name from
     * @returns The base name of the path
     */
    function base(path: string): string;

    /**
     * Cleans the path by applying a series of rules.
     * @param path The path to clean
     * @returns The cleaned path
     */
    function clean(path: string): string;

    /**
     * Returns all but the last element of path.
     * @param path The path to get the directory from
     * @returns The directory containing the file
     */
    function dir(path: string): string;

    /**
     * Returns the file extension of path.
     * @param path The path to get the extension from
     * @returns The file extension (including the dot)
     */
    function ext(path: string): string;

    /**
     * Converts path from slash-separated to OS-specific separator.
     * @param path The path to convert
     * @returns The path with OS-specific separators
     */
    function fromSlash(path: string): string;

    /**
     * Returns a list of files matching the pattern in the base directory.
     * @param basePath The base directory to search in
     * @param pattern The glob pattern to match
     * @returns An array of matching file paths
     * @throws Error if the base path is not authorized for reading
     */
    function glob(basePath: string, pattern: string): string[];

    /**
     * Reports whether the path is absolute.
     * @param path The path to check
     * @returns True if the path is absolute
     */
    function isAbs(path: string): boolean;

    /**
     * Joins any number of path elements into a single path.
     * @param paths The path elements to join
     * @returns The joined path
     */
    function join(...paths: string[]): string;

    /**
     * Reports whether name matches the shell pattern.
     * @param pattern The pattern to match against
     * @param name The string to check
     * @returns True if name matches pattern
     */
    function match(pattern: string, name: string): boolean;

    /**
     * Returns the relative path from basepath to targpath.
     * @param basepath The base path
     * @param targpath The target path
     * @returns The relative path
     */
    function rel(basepath: string, targpath: string): string;

    /**
     * Splits path into directory and file components.
     * @param path The path to split
     * @returns An array with [directory, file]
     */
    function split(path: string): [string, string];

    /**
     * Splits a list of paths joined by the OS-specific ListSeparator.
     * @param path The path list to split
     * @returns An array of paths
     */
    function splitList(path: string): string[];

    /**
     * Converts path from OS-specific separator to slash-separated.
     * @param path The path to convert
     * @returns The path with forward slashes
     */
    function toSlash(path: string): string;

    /**
     * Walks the file tree rooted at root, calling walkFn for each file or directory.
     * @param root The root directory to start walking from
     * @param walkFn The function to call for each file or directory
     * @throws Error if the root path is not authorized for reading
     */
    function walk(root: string, walkFn: (path: string, info: FileInfo, err: Error | null) => void): void;

    /**
     * Walks the file tree rooted at root, calling walkDirFn for each file or directory.
     * @param root The root directory to start walking from
     * @param walkDirFn The function to call for each file or directory
     * @throws Error if the root path is not authorized for reading
     */
    function walkDir(root: string, walkDirFn: (path: string, d: DirEntry, err: Error | null) => void): void;
}

/**
 * Extra OS utilities not in the standard library.
 */
declare namespace $osExtra {
    /**
     * Unwraps an archive and moves its contents to the destination.
     * @param src The source archive path
     * @param dest The destination directory
     * @throws Error if either path is not authorized for writing
     */
    function unwrapAndMove(src: string, dest: string): void;

    /**
     * Extracts a ZIP archive to the destination directory.
     * @param src The source ZIP file path
     * @param dest The destination directory
     * @throws Error if either path is not authorized for writing
     */
    function unzip(src: string, dest: string): void;

    /**
     * Extracts a RAR archive to the destination directory.
     * @param src The source RAR file path
     * @param dest The destination directory
     * @throws Error if either path is not authorized for writing
     */
    function unrar(src: string, dest: string): void;
}

/**
 * Downloader module for downloading files with progress tracking.
 */
declare namespace $downloader {
    /**
     * Download status constants
     */
    enum DownloadStatus {
        DOWNLOADING = "downloading",
        COMPLETED = "completed",
        CANCELLED = "cancelled",
        ERROR = "error"
    }

    /**
     * Download progress information
     */
    interface DownloadProgress {
        /** Unique download identifier */
        id: string;
        /** Source URL */
        url: string;
        /** Destination file path */
        destination: string;
        /** Number of bytes downloaded so far */
        totalBytes: number;
        /** Total file size in bytes (if known) */
        totalSize: number;
        /** Download speed in bytes per second */
        speed: number;
        /** Download completion percentage (0-100) */
        percentage: number;
        /** Current download status */
        status: DownloadStatus;
        /** Error message if status is ERROR */
        error?: string;
        /** Time of the last progress update */
        lastUpdate: Date;
        /** Time when the download started */
        startTime: Date;
    }

    /**
     * Download options
     */
    interface DownloadOptions {
        /** Timeout in seconds */
        timeout?: number;
        /** HTTP headers to send with the request */
        headers?: Record<string, string>;
    }

    /**
     * Starts a file download.
     * @param url The URL to download from
     * @param destination The path to save the file to
     * @param options Download options
     * @returns A unique download ID
     * @throws Error if the destination path is not authorized for writing
     */
    function download(url: string, destination: string, options?: DownloadOptions): string;

    /**
     * Watches a download for progress updates.
     * @param downloadId The download ID to watch
     * @param callback Function to call with progress updates
     * @returns A function to cancel the watch
     */
    function watch(downloadId: string, callback: (progress: DownloadProgress) => void): () => void;

    /**
     * Gets the current progress of a download.
     * @param downloadId The download ID to check
     * @returns The current download progress
     */
    function getProgress(downloadId: string): DownloadProgress | undefined;

    /**
     * Lists all active downloads.
     * @returns An array of download progress objects
     */
    function listDownloads(): DownloadProgress[];

    /**
     * Cancels a specific download.
     * @param downloadId The download ID to cancel
     * @returns True if the download was cancelled
     */
    function cancel(downloadId: string): boolean;

    /**
     * Cancels all active downloads.
     * @returns The number of downloads cancelled
     */
    function cancelAll(): number;
}

/**
 * MIME type utilities.
 */
declare namespace $mime {
    /**
     * Parses a MIME type string and returns the media type and parameters.
     * @param contentType The MIME type string to parse
     * @returns An object containing the media type and parameters
     * @throws Error if parsing fails
     */
    function parse(contentType: string): { mediaType: string; parameters: Record<string, string> };
}
