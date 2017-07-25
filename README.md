A guide for 'hacking' your ikea dioders with a Go webserver.
===========
First of all, this is not my work. Full credit goes to lakrizz! This fork is made so people can find the guide which is lost due to the new website that replace. Check his new site out on https://krizzblog.de or go to his github page! 

The guide that I'm referring to can be found here as it's cached on google. Check it out on this url: http://webcache.googleusercontent.com/search?q=cache:OUkKSCsh4ewJ:krizzblog.de/2017/02/20/pidioder-tutorial/

Also in this github is his original code which I translated some of it to english.


To install Go which is needed to run this project, enter the following lines in the terminal:
    wget https://storage.googleapis.com/golang/go1.8.3.linux-armv6l.tar.gz
    
    sudo tar -C /usr/local -xzf go1.8.3.linux-armv6l.tar.gz

Then add this line to ~/.profile by doing this:
    sudo nano ~/.profile
And add this line to the top
    export PATH=$PATH:/usr/local/go/bin
Save the file. 

If the last instructions don't work, you can also enter this line every time you want to use the Go server:
    PATH=$PATH:/usr/local/go/bin
