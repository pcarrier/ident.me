plugins {
    id("com.android.application").version("8.2.0-alpha01")
    id("org.jetbrains.kotlin.android").version("1.8.21")
}

repositories {
    google()
    mavenCentral()
}

android {
    namespace = "me.ident"
    compileSdk = 23
    defaultConfig {
        minSdk = 23
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }

    buildTypes {
        getByName("release") {
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"))
        }
    }
}

kotlin {
    jvmToolchain(8)
}
