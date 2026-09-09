import Navbar from "@/components/Navbar";
import Hero from "@/components/Hero";
import Features from "@/components/Features";
import CTA from "@/components/CTA";
import Footer from "@/components/Footer";

export default function Page() {
  return (
    <>
      <Navbar />
      <main className="w-full bg-background">
        <Hero />
        <Features />
        <CTA />
      </main>
      <Footer />
    </>
  );
}