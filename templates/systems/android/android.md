# apktool

    paru -S android-apktool


Decode the whole apk:

    apktool decode -o dir file.apk



# resources.arsc

Decompile and list resources.arsc like this:

    aapt dump resources app.apk   # paru -S android-sdk-build-tools


https://stackoverflow.com/questions/27548810/android-compiled-resources-resources-arsc



# APK-Analyzer

https://developer.android.com/studio/debug/apk-analyzer


# dex2jar

Converts apk to a jar archive

    paru -S dex2jar

    dex2jar sampleApp.apk

# ghidra

Ghidra can import .apk or .jar files

