export default function Home() {
  return (
    <main className="flex min-h-screen items-center justify-center">
      <div className="text-center">
        <h1 className="text-4xl font-bold text-primary-600 mb-4">
          {process.env.NEXT_PUBLIC_APP_NAME || 'StressTestPlatform'}
        </h1>
        <p className="text-lg text-gray-500">
          Projeto: {process.env.NEXT_PUBLIC_PROJECT_NAME || 'BR-IDNF'}
        </p>
        <p className="mt-2 text-gray-400">
          Plataforma de testes de carga
        </p>
      </div>
    </main>
  )
}
