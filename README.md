//NAME: Grant Thompson
//NETID: gwt5035

=======================
Build/Run Instructions:
=======================

    make clean && make build

This will produce a top level executable named "412alloc"


======
Usage:
======

    ./schedule -h
    .schedule <iloc file>



===============================
Command Line Options Explained:
===============================

    -h
        Prints a help/usage message and exits.
    <file>
        Scans/parses <file>, renames SRs to VRs, and runs the scheduler on the resulting basic block. The scheduled ILOC is printed to stdout.



================
Flags Explained:
================

If multiple flags are provided, they are prioritized in order of -h, else <file>.
if no flag/file is given, or the file cannot be opened, the program terminates and reports the error.


    

