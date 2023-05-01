package me.ident

import android.app.Activity
import android.graphics.Typeface
import android.os.Bundle
import android.util.TypedValue
import android.view.Gravity
import android.widget.Button
import android.widget.LinearLayout
import android.widget.TextView

class MainActivity : Activity() {
    private fun title(of: String) =
        TextView(this).apply {
            text = of
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 18f)
            setTypeface(null, Typeface.BOLD)
            gravity = Gravity.CENTER
        }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        fun refresh() {
            println("Refreshing")
        }

        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setGravity(Gravity.CENTER)
            addView(title("IPv4"))
            addView(title("IPv6"))
            addView(Button(this@MainActivity).apply {
                text = "Refresh"
                setOnClickListener {
                    refresh()
                }
            })
        }
        setContentView(layout)
    }
}
