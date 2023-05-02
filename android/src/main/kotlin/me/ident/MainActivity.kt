package me.ident

import android.app.Activity
import android.content.Context
import android.graphics.Typeface
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.view.View
import android.view.Window
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject
import java.io.BufferedInputStream
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLConnection

data class Ident(
        val ip: String,
        val aso: String,
        val postal: String,
        val city: String,
        val country: String,
        val latitude: Double,
        val longitude: Double
)

class MainActivity : Activity() {
    private val v4url = URL("https://4.ident.me/json")
    private val v6url = URL("https://6.ident.me/json")

    private val v4: Ident? = null
    private val v6: Ident? = null

    private fun title(of: String) =
            TextView(this).apply {
                text = of
                setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f)
                setTypeface(null, Typeface.BOLD)
                gravity = Gravity.CENTER
            }

    private fun fetch(url: URL): Ident {
        val conn = url.openConnection() as HttpURLConnection
        try {
            conn.inputStream.use {
                val obj = JSONObject(String(it.readBytes()))
                return Ident(
                        ip = obj.getString("ip"),
                        aso = obj.getString("aso"),
                        postal = obj.getString("postal"),
                        city = obj.getString("city"),
                        country = obj.getString("country"),
                        latitude = obj.getString("latitude").toDouble(),
                        longitude = obj.getString("longitude").toDouble(),

                )
            }
        } finally {
            conn.disconnect()
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        requestWindowFeature(Window.FEATURE_NO_TITLE)

        fun refresh() {
            println("Refreshing")
        }

        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setGravity(Gravity.CENTER)
            addView(title("IPv4"))
            addView(LinearLayout(this@MainActivity).apply {
                addView(Button(this@MainActivity).apply {

                    text = "Copy"
                })
            })
            addView(title("IPv6"))
            addView(Button(this@MainActivity).apply {
                text = "Refresh"
                setOnClickListener {
                    refresh()
                }
            })
        }

        setContentView(layout)
        refresh()
    }
}
