# David's Dumb Thinkpad Power (ddtp) Program

Some random code to play around with various pieces of power management (CPU undervolting,
thermal management, etc.)

### *Warning*:

You absolutely, positively, definitely should not run this on any computer anywhere in its
current state. This reads and sets CPU MSRs with reckless abandon and could reduce any computer
to a smoldering heap of metal, plastic, and failed dreams.

### Prior Work

The temperature and TDP wattage is well-documented from Intel, so that was a straightforward
application. The voltage offset application stuff is another matter entirely; there's really
not much work out there.

I found these two projects exceptionally helpful:

* https://github.com/georgewhewell/undervolt/
* https://github.com/mihic/linux-intel-undervolt

Particularly, I've taken some test cases and logic from George Whewell's repo as he's put a ton
of work into this problem.